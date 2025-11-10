package plugin

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/discovery"
	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/logger"
	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/scaling"
	v1 "k8s.io/api/core/v1"
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

	pods, err := findPodsForStorageClass(ctx, log, finder, flags)
	if err != nil {
		return err
	}

	if len(pods) == 0 {
		log.Info("No pods found, nothing to do")
		return nil
	}

	log.Info("Found %d pods to scale down", len(pods))
	for _, pod := range pods {
		fmt.Printf("%s/%s\n", pod.Namespace, pod.Name)
	}

	skipConfirmation := flags.Confirmed != nil && *flags.Confirmed
	confirmed, err := confirmAction(log, "Scale down the pods listed above?", skipConfirmation)
	if err != nil {
		return err
	}
	if !confirmed {
		log.Info("Operation cancelled by user")
		return nil
	}

	controllers, err := finder.FindControllers(ctx, pods)
	if err != nil {
		return err
	}

	if len(controllers) == 0 {
		log.Info("No controllers found to scale down")
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

	log.Info("Scale down complete")

	// TODO: show spinner and wait until scaled down. Check for pods in the list until not found anymore (or time out).

	return nil
}

func findPodsForStorageClass(ctx context.Context, log *logger.Logger, finder discovery.Finder, flags *ConfigFlags) ([]v1.Pod, error) {
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
		return nil, err
	}

	if len(pvcsPerNs) == 0 {
		log.Info("No matching PVCs found, nothing to do")
		return nil, nil
	}

	log.Info("Finding pods...")
	return finder.FindPodsUsingPVCs(ctx, pvcsPerNs)
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
