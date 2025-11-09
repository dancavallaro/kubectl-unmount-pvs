package plugin

import (
	"bufio"
	"context"
	"fmt"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/logger"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

// controllerRef represents a Kubernetes controller that owns a pod
type controllerRef struct {
	Kind      string
	Namespace string
	Name      string
}

// findTopLevelController traces the owner references to find the top-level controller.
// It walks up the ownership chain (e.g., Pod -> ReplicaSet -> Deployment).
func findTopLevelController(ctx context.Context, clientset *kubernetes.Clientset, pod v1.Pod) (*controllerRef, error) {
	// Check if pod has any owner references
	if len(pod.OwnerReferences) == 0 {
		// Standalone pod with no controller
		return &controllerRef{
			Kind:      "Pod",
			Namespace: pod.Namespace,
			Name:      pod.Name,
		}, nil
	}

	// Get the first owner reference (typically there's only one)
	owner := pod.OwnerReferences[0]

	// If the owner is a ReplicaSet, check if it has a Deployment owner
	if owner.Kind == "ReplicaSet" {
		rs, err := clientset.AppsV1().ReplicaSets(pod.Namespace).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			// ReplicaSet might have been deleted, use it as the controller
			return &controllerRef{
				Kind:      owner.Kind,
				Namespace: pod.Namespace,
				Name:      owner.Name,
			}, nil
		}

		// Check if ReplicaSet has an owner (likely a Deployment)
		if len(rs.OwnerReferences) > 0 {
			rsOwner := rs.OwnerReferences[0]
			return &controllerRef{
				Kind:      rsOwner.Kind,
				Namespace: pod.Namespace,
				Name:      rsOwner.Name,
			}, nil
		}

		// ReplicaSet has no owner, it's the top-level controller
		return &controllerRef{
			Kind:      "ReplicaSet",
			Namespace: pod.Namespace,
			Name:      rs.Name,
		}, nil
	}

	// For other controller types (StatefulSet, DaemonSet, etc.), return as-is
	return &controllerRef{
		Kind:      owner.Kind,
		Namespace: pod.Namespace,
		Name:      owner.Name,
	}, nil
}

// scaleDownController scales down the given controller to 0 replicas.
func scaleDownController(ctx context.Context, log *logger.Logger, clientset *kubernetes.Clientset, ctrl *controllerRef) error {
	zero := int32(0)

	switch ctrl.Kind {
	case "Deployment":
		deployment, err := clientset.AppsV1().Deployments(ctrl.Namespace).Get(ctx, ctrl.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get deployment %s/%s: %w", ctrl.Namespace, ctrl.Name, err)
		}
		if deployment.Spec.Replicas != nil && *deployment.Spec.Replicas == 0 {
			log.Info("Deployment %s/%s is already scaled to 0", ctrl.Namespace, ctrl.Name)
			return nil
		}
		deployment.Spec.Replicas = &zero
		_, err = clientset.AppsV1().Deployments(ctrl.Namespace).Update(ctx, deployment, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to scale down deployment %s/%s: %w", ctrl.Namespace, ctrl.Name, err)
		}
		log.Info("Scaled down Deployment %s/%s to 0 replicas", ctrl.Namespace, ctrl.Name)

	case "StatefulSet":
		statefulSet, err := clientset.AppsV1().StatefulSets(ctrl.Namespace).Get(ctx, ctrl.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get statefulset %s/%s: %w", ctrl.Namespace, ctrl.Name, err)
		}
		if statefulSet.Spec.Replicas != nil && *statefulSet.Spec.Replicas == 0 {
			log.Info("StatefulSet %s/%s is already scaled to 0", ctrl.Namespace, ctrl.Name)
			return nil
		}
		statefulSet.Spec.Replicas = &zero
		_, err = clientset.AppsV1().StatefulSets(ctrl.Namespace).Update(ctx, statefulSet, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to scale down statefulset %s/%s: %w", ctrl.Namespace, ctrl.Name, err)
		}
		log.Info("Scaled down StatefulSet %s/%s to 0 replicas", ctrl.Namespace, ctrl.Name)

	case "ReplicaSet":
		replicaSet, err := clientset.AppsV1().ReplicaSets(ctrl.Namespace).Get(ctx, ctrl.Name, metav1.GetOptions{})
		if err != nil {
			return fmt.Errorf("failed to get replicaset %s/%s: %w", ctrl.Namespace, ctrl.Name, err)
		}
		if replicaSet.Spec.Replicas != nil && *replicaSet.Spec.Replicas == 0 {
			log.Info("ReplicaSet %s/%s is already scaled to 0", ctrl.Namespace, ctrl.Name)
			return nil
		}
		replicaSet.Spec.Replicas = &zero
		_, err = clientset.AppsV1().ReplicaSets(ctrl.Namespace).Update(ctx, replicaSet, metav1.UpdateOptions{})
		if err != nil {
			return fmt.Errorf("failed to scale down replicaset %s/%s: %w", ctrl.Namespace, ctrl.Name, err)
		}
		log.Info("Scaled down ReplicaSet %s/%s to 0 replicas", ctrl.Namespace, ctrl.Name)

	case "Pod":
		// Standalone pod - delete it directly
		err := clientset.CoreV1().Pods(ctrl.Namespace).Delete(ctx, ctrl.Name, metav1.DeleteOptions{})
		if err != nil {
			return fmt.Errorf("failed to delete pod %s/%s: %w", ctrl.Namespace, ctrl.Name, err)
		}
		log.Info("Deleted standalone Pod %s/%s", ctrl.Namespace, ctrl.Name)

	case "DaemonSet":
		log.Warn("Cannot scale down DaemonSet %s/%s (DaemonSets cannot be scaled)", ctrl.Namespace, ctrl.Name)
		return nil

	default:
		log.Warn("Unsupported controller type %s for %s/%s, skipping", ctrl.Kind, ctrl.Namespace, ctrl.Name)
		return nil
	}

	return nil
}

func run(ctx context.Context, log *logger.Logger, flags *ConfigFlags, clientset *kubernetes.Clientset) error {
	log.Info("Finding volumes...")
	pvcsPerNs, err := discoverPVCs(ctx, log, clientset, flags)
	if err != nil {
		return err
	}

	if len(pvcsPerNs) == 0 {
		log.Info("No matching PVCs found, nothing to do")
		return nil
	}

	log.Info("Finding pods...")
	pods, err := findPodsUsingPVCs(ctx, clientset, pvcsPerNs)
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

	// Find controllers for all pods and deduplicate
	log.Info("Finding controllers for pods...")
	controllerMap := make(map[string]*controllerRef) // key: "kind/namespace/name"
	for _, pod := range pods {
		ctrl, err := findTopLevelController(ctx, clientset, pod)
		if err != nil {
			log.Warn("Failed to find controller for pod %s/%s: %v", pod.Namespace, pod.Name, err)
			continue
		}
		key := fmt.Sprintf("%s/%s/%s", ctrl.Kind, ctrl.Namespace, ctrl.Name)
		controllerMap[key] = ctrl
	}

	if len(controllerMap) == 0 {
		log.Info("No controllers found to scale down")
		return nil
	}

	// Scale down each unique controller
	log.Info("Scaling down %d controller(s)...", len(controllerMap))
	for _, ctrl := range controllerMap {
		if *flags.DryRun {
			log.Info("(dry-run, skipping controller: %s/%s/%s)", ctrl.Namespace, ctrl.Kind, ctrl.Name)
			continue
		}
		if err := scaleDownController(ctx, log, clientset, ctrl); err != nil {
			log.Warn("Failed to scale down %s %s/%s: %v", ctrl.Kind, ctrl.Namespace, ctrl.Name, err)
			// Continue with other controllers even if one fails
		}
	}

	log.Info("Scale down complete")

	// TODO: show spinner and wait until scaled down. Check for pods in the list until not found anymore (or time out).

	return nil
}

// matchesStorageClass checks if a storage class name matches the filter.
// Returns true if no filter is set or if the names match.
func matchesStorageClass(storageClassName *string, filter string) bool {
	if filter == "" {
		return true
	}
	if storageClassName == nil {
		return false
	}
	return *storageClassName == filter
}

// discoverPVCs finds all PVCs that match the given filters.
// Returns a map from namespace to list of PVC names.
func discoverPVCs(ctx context.Context, log *logger.Logger, clientset *kubernetes.Clientset, flags *ConfigFlags) (map[string][]string, error) {
	pvcsPerNs := make(map[string][]string)
	storageClassFilter := ""
	if flags.StorageClass != nil {
		storageClassFilter = *flags.StorageClass
	}

	if *flags.Namespace != "" {
		// Search PVCs in the specified namespace
		pvcList, err := clientset.CoreV1().PersistentVolumeClaims(*flags.Namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list persistent volume claims: %w", err)
		}
		for _, pvc := range pvcList.Items {
			if !matchesStorageClass(pvc.Spec.StorageClassName, storageClassFilter) {
				continue
			}
			pvcsPerNs[pvc.Namespace] = append(pvcsPerNs[pvc.Namespace], pvc.Name)
		}
	} else {
		// Search PVs across all namespaces
		pvList, err := clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list persistent volumes: %w", err)
		}
		for _, pv := range pvList.Items {
			if !matchesStorageClass(&pv.Spec.StorageClassName, storageClassFilter) {
				continue
			}
			if pv.Spec.ClaimRef == nil {
				log.Warn("PersistentVolume %s has no ClaimRef, skipping", pv.Name)
				continue
			}
			namespace := pv.Spec.ClaimRef.Namespace
			name := pv.Spec.ClaimRef.Name
			pvcsPerNs[namespace] = append(pvcsPerNs[namespace], name)
		}
	}

	return pvcsPerNs, nil
}

// findPodsUsingPVCs finds all pods that are using the given PVCs.
// Returns a deduplicated list of pods.
func findPodsUsingPVCs(ctx context.Context, clientset *kubernetes.Clientset, pvcsPerNs map[string][]string) ([]v1.Pod, error) {
	pods := make(map[string]v1.Pod) // key: namespace/name

	for ns, pvcs := range pvcsPerNs {
		podList, err := clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list pods: %w", err)
		}

		for _, pod := range podList.Items {
			if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
				continue
			}
			for _, vol := range pod.Spec.Volumes {
				if vol.PersistentVolumeClaim != nil && slices.Contains(pvcs, vol.PersistentVolumeClaim.ClaimName) {
					key := fmt.Sprintf("%s/%s", pod.Namespace, pod.Name)
					pods[key] = pod
					break // Found a matching PVC, no need to check other volumes
				}
			}
		}
	}

	return slices.Collect(maps.Values(pods)), nil
}
