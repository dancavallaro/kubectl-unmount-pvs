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
	if filter.Namespace != "" {
		// Search PVCs in the specified namespace
		return f.findPVCsInNamespace(ctx, filter.Namespace, filter.StorageClass)
	}

	// Search across all namespaces by listing PVs
	return f.findPVCsFromPVs(ctx, filter.StorageClass)
}

// findPVCsInNamespace finds PVCs in a specific namespace.
func (f *Finder) findPVCsInNamespace(ctx context.Context, namespace, storageClass string) (map[string][]string, error) {
	pvcsPerNs := make(map[string][]string)

	pvcList, err := f.clientset.CoreV1().PersistentVolumeClaims(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list persistent volume claims: %w", err)
	}

	for _, pvc := range pvcList.Items {
		if !matchesStorageClass(pvc.Spec.StorageClassName, storageClass) {
			continue
		}
		pvcsPerNs[pvc.Namespace] = append(pvcsPerNs[pvc.Namespace], pvc.Name)
	}

	return pvcsPerNs, nil
}

// findPVCsFromPVs finds PVCs across all namespaces by listing PVs.
func (f *Finder) findPVCsFromPVs(ctx context.Context, storageClass string) (map[string][]string, error) {
	pvcsPerNs := make(map[string][]string)

	pvList, err := f.clientset.CoreV1().PersistentVolumes().List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list persistent volumes: %w", err)
	}

	for _, pv := range pvList.Items {
		if !matchesStorageClass(&pv.Spec.StorageClassName, storageClass) {
			continue
		}
		if pv.Spec.ClaimRef == nil {
			f.log.Warn("PersistentVolume %s has no ClaimRef, skipping", pv.Name)
			continue
		}
		namespace := pv.Spec.ClaimRef.Namespace
		name := pv.Spec.ClaimRef.Name
		pvcsPerNs[namespace] = append(pvcsPerNs[namespace], name)
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
