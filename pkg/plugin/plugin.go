package plugin

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/discovery"
	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/logger"
	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/scaling"
	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/spinner"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
)

type ConfigFlags struct {
	genericclioptions.ConfigFlags

	Confirmed    *bool
	DryRun       *bool
	StorageClass *string
}

func RunPlugin(flags *ConfigFlags) error {
	ctx := context.Background()
	log := logger.NewLogger(os.Stderr)

	config, err := flags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	return run(ctx, log, flags, clientset)
}

func run(ctx context.Context, log *logger.Logger, flags *ConfigFlags, clientset *kubernetes.Clientset) error {
	finder := discovery.New(clientset, log)

	filter := discovery.PVCFilter{}
	if flags.Namespace != nil {
		filter.Namespace = *flags.Namespace
	}
	if flags.StorageClass != nil {
		filter.StorageClass = *flags.StorageClass
	}

	log.Info("Finding volumes...")
	pvcsPerNs, err := finder.FindPVCs(ctx, filter)
	if err != nil {
		return err
	}
	if len(pvcsPerNs) == 0 {
		log.Info("No matching PVCs found, nothing to do")
		return nil
	}

	log.Info("Finding pods...")
	pods, err := finder.FindPodsUsingPVCs(ctx, pvcsPerNs)
	if err != nil {
		return err
	}
	if len(pods) == 0 {
		log.Info("No pods found, nothing to do")
		return nil
	}
	log.Info("Found %d pods to scale down", len(pods))

	controllers, err := finder.FindControllers(ctx, pods)
	if err != nil {
		return err
	}
	if len(controllers) == 0 {
		log.Info("No controllers found to scale down")
		return nil
	}
	log.Info("Found %d controllers to scale down", len(controllers))

	// Print the affected controllers on stdout (other logs are on stderr)
	for _, controller := range controllers {
		fmt.Printf("  %v\n", controller)
	}

	skipConfirmation := flags.Confirmed != nil && *flags.Confirmed
	confirmed, err := confirmAction(log, "Scale down the controllers listed above?", skipConfirmation)
	if err != nil {
		return err
	}
	if !confirmed {
		log.Info("Operation cancelled by user")
		return nil
	}

	scaler := scaling.New(clientset, log, *flags.DryRun)
	log.Info("Scaling down %d controller(s)...", len(controllers))
	for _, ctrl := range controllers {
		if err := scaler.ScaleDown(ctx, ctrl); err != nil {
			log.Error(err)
			// Continue with other controllers even if one fails
		}
	}

	if !*flags.DryRun {
		<-spinner.Wait("Waiting for pods to scale down... ", func() (bool, error) {
			pods, err := finder.FindPodsUsingPVCs(ctx, pvcsPerNs)
			if err != nil {
				return false, err
			}
			return len(pods) == 0, nil
		}, func(err error) {
			log.Error(err)
		}, 2*time.Second)
	}

	log.Info("Scale down complete")

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
