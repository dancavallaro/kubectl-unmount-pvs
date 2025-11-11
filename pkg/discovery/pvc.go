package discovery

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PVCFilter contains criteria for filtering PVCs during discovery.
type PVCFilter struct {
	Namespace    string
	StorageClass string
}

// FindPVCs discovers all PVCs that match the given filters.
// Returns a map from namespace to list of PVC names.
func (f *Finder) FindPVCs(ctx context.Context, filter PVCFilter) (map[string][]string, error) {
	pvcsPerNs := make(map[string][]string)

	pvcList, err := f.clientset.CoreV1().PersistentVolumeClaims(filter.Namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list persistent volume claims: %w", err)
	}

	for _, pvc := range pvcList.Items {
		if !matchesStorageClass(pvc.Spec.StorageClassName, filter.StorageClass) {
			continue
		}
		pvcsPerNs[pvc.Namespace] = append(pvcsPerNs[pvc.Namespace], pvc.Name)
	}

	return pvcsPerNs, nil
}

func matchesStorageClass(storageClassName *string, filter string) bool {
	if filter == "" {
		return true
	}
	if storageClassName == nil {
		return false
	}
	return *storageClassName == filter
}
