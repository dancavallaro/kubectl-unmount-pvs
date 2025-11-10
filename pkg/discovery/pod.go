package discovery

import (
	"context"
	"fmt"
	"maps"
	"slices"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// FindPodsUsingPVCs finds all pods that are using the given PVCs.
// Returns a deduplicated list of pods.
func (f *Finder) FindPodsUsingPVCs(ctx context.Context, pvcsPerNs map[string][]string) ([]v1.Pod, error) {
	pods := make(map[string]v1.Pod) // key: namespace/name

	for ns, pvcs := range pvcsPerNs {
		podList, err := f.clientset.CoreV1().Pods(ns).List(ctx, metav1.ListOptions{})
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
