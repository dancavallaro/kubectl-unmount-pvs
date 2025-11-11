package discovery

import (
	"context"
	"maps"
	"slices"

	"github.com/dancavallaro/kubectl-unmount/pkg/common"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FindController traces the owner references to find the top-level controller.
// It walks up the ownership chain (e.g., Pod -> ReplicaSet -> Deployment).
func (f *Finder) FindController(ctx context.Context, pod corev1.Pod) (common.ControllerRef, error) {
	// Check if pod has any owner references
	if len(pod.OwnerReferences) == 0 {
		// Standalone pod with no controller
		return common.ControllerRef{
			Kind:      common.KindPod,
			Namespace: pod.Namespace,
			Name:      pod.Name,
		}, nil
	}

	// Get the first owner reference (typically there's only one)
	owner := pod.OwnerReferences[0]

	// If the owner is a ReplicaSet, check if it has a Deployment owner
	if owner.Kind == common.KindReplicaSet {
		rs, err := f.clientset.AppsV1().ReplicaSets(pod.Namespace).Get(ctx, owner.Name, metav1.GetOptions{})
		if err != nil {
			return common.ControllerRef{}, err
		}

		// Check if ReplicaSet has an owner (likely a Deployment)
		if len(rs.OwnerReferences) > 0 {
			rsOwner := rs.OwnerReferences[0]
			return common.ControllerRef{
				Kind:      rsOwner.Kind,
				Namespace: pod.Namespace,
				Name:      rsOwner.Name,
			}, nil
		}

		// ReplicaSet has no owner, it's the top-level controller
		return common.ControllerRef{
			Kind:      common.KindReplicaSet,
			Namespace: pod.Namespace,
			Name:      rs.Name,
		}, nil
	}

	// For other controller types (StatefulSet, DaemonSet, etc.), return as-is
	return common.ControllerRef{
		Kind:      owner.Kind,
		Namespace: pod.Namespace,
		Name:      owner.Name,
	}, nil
}

// FindControllers finds the (deduplicated) top-level controllers for the provided pods.
func (f *Finder) FindControllers(ctx context.Context, pods []corev1.Pod) ([]common.ControllerRef, error) {
	f.log.Info("Finding controllers for pods...")
	controllers := make(map[string]common.ControllerRef) // key: "kind/namespace/name"
	for _, pod := range pods {
		ctrl, err := f.FindController(ctx, pod)
		if err != nil {
			f.log.Warn("Failed to find controller for pod %s/%s: %v", pod.Namespace, pod.Name, err)
			return nil, err
		}
		controllers[ctrl.String()] = ctrl
	}

	return slices.Collect(maps.Values(controllers)), nil
}
