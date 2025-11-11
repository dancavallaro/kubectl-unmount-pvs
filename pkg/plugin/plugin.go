package plugin

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	"github.com/dancavallaro/kubectl-unmount/pkg/discovery"
	"github.com/dancavallaro/kubectl-unmount/pkg/logger"
	"github.com/dancavallaro/kubectl-unmount/pkg/scaling"
	"github.com/dancavallaro/kubectl-unmount/pkg/spinner"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

type ConfigFlags struct {
	genericclioptions.ConfigFlags

	Confirmed    *bool
	DryRun       *bool
	StorageClass *string
	PVCName      *string

	logger *logger.Logger
	out    io.Writer
}

func RunPlugin(pluginCfg *ConfigFlags) error {
	ctx := context.Background()
	if pluginCfg.logger == nil {
		pluginCfg.logger = logger.NewLogger(os.Stderr)
	}
	if pluginCfg.out == nil {
		pluginCfg.out = os.Stdout
	}

	config, err := pluginCfg.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	return run(ctx, pluginCfg, clientset)
}

func run(ctx context.Context, cfg *ConfigFlags, clientset *kubernetes.Clientset) error {
	finder := discovery.New(clientset, cfg.logger)

	filter := discovery.PVCFilter{}
	if cfg.Namespace != nil {
		filter.Namespace = *cfg.Namespace
	}
	if cfg.StorageClass != nil {
		filter.StorageClass = *cfg.StorageClass
	}

	cfg.logger.Info("Finding volumes...")
	var pvcsPerNs map[string][]string
	if *cfg.PVCName == "" {
		var err error
		pvcsPerNs, err = finder.FindPVCs(ctx, filter)
		if err != nil {
			return err
		}
		if len(pvcsPerNs) == 0 {
			cfg.logger.Info("No matching PVCs found, nothing to do")
			return nil
		}
	} else {
		pvcsPerNs = map[string][]string{
			*cfg.Namespace: {*cfg.PVCName},
		}
	}

	cfg.logger.Info("Finding pods...")
	pods, err := finder.FindPodsUsingPVCs(ctx, pvcsPerNs)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		cfg.logger.Info("No pods found, nothing to do")
		return nil
	}
	cfg.logger.Info("Found %d pods to scale down", len(pods))

	controllers, err := finder.FindControllers(ctx, pods)
	if err != nil {
		return err
	}
	if len(controllers) == 0 {
		cfg.logger.Info("No controllers found to scale down")
		return nil
	}
	cfg.logger.Info("Found %d controllers to scale down", len(controllers))

	// Print the affected controllers on stdout (other logs are on stderr)
	for _, controller := range controllers {
		_, _ = fmt.Fprintf(cfg.out, "  %v\n", controller)
	}

	skipConfirmation := cfg.Confirmed != nil && *cfg.Confirmed
	confirmed, err := confirmAction(cfg.logger, "Scale down the controllers listed above?", skipConfirmation)
	if err != nil {
		return err
	}
	if !confirmed {
		cfg.logger.Info("Operation cancelled by user")
		return nil
	}

	cfg.logger.Info("Scaling down %d controller(s)...", len(controllers))
	scaler := scaling.New(clientset, cfg.logger, *cfg.DryRun)
	errors := 0
	for _, ctrl := range controllers {
		if err := scaler.ScaleDown(ctx, ctrl); err != nil {
			cfg.logger.Error(err)
			errors++
			// Continue with other controllers even if one fails
		}
	}

	if errors > 0 {
		return fmt.Errorf("encountered %d errors scaling down", errors)
	}

	if !*cfg.DryRun {
		<-spinner.Wait("Waiting for pods to scale down... ", func() (bool, error) {
			pods, err := finder.FindPodsUsingPVCs(ctx, pvcsPerNs)
			if err != nil {
				return false, err
			}
			return len(pods) == 0, nil
		}, func(err error) {
			cfg.logger.Error(err)
		}, 2*time.Second)
	}

	cfg.logger.Info("Scale down complete")

	return nil
}

// confirmAction prompts the user to confirm an action by typing "yes".
// Returns true if the user confirms, false otherwise.
func confirmAction(log *logger.Logger, prompt string, skipConfirmation bool) (bool, error) {
	if skipConfirmation {
		return true, nil
	}

	log.Instructions("%s\nType 'yes' to continue: ", prompt)

	reader := bufio.NewReader(os.Stdin)
	response, err := reader.ReadString('\n')
	if err != nil {
		return false, fmt.Errorf("failed to read user input: %w", err)
	}

	response = strings.TrimSpace(strings.ToLower(response))
	return response == "yes", nil
}
