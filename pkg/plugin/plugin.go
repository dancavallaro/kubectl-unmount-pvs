package plugin

import (
	"fmt"
	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/logger"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/client-go/kubernetes"
	"slices"
)

type ConfigFlags struct {
	genericclioptions.ConfigFlags
	StorageClass *string
}

func RunPlugin(flags *ConfigFlags) error {
	log := logger.NewLogger()

	config, err := flags.ToRESTConfig()
	if err != nil {
		return fmt.Errorf("failed to read kubeconfig: %w", err)
	}

	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return fmt.Errorf("failed to create clientset: %w", err)
	}

	return run(log, flags, clientset)
}

func run(log *logger.Logger, flags *ConfigFlags, clientset *kubernetes.Clientset) error {
	// If a namespace is provided, just search PVCs in that namespace and filter by SC (if provided)
	// If no ns provided, list PVs (in all namespaces), filter by SC (if provided), and collect associated PVCs
	log.Info("Finding volumes...")
	pvcsPerNs := make(map[string][]string) // map from namespace to filtered list of PVC names
	if *flags.Namespace != "" {
		pvcList, err := clientset.CoreV1().PersistentVolumeClaims(*flags.Namespace).List(metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list persistent volume claims: %w", err)
		}
		for _, pvc := range pvcList.Items {
			if *flags.StorageClass != "" && *pvc.Spec.StorageClassName != *flags.StorageClass {
				continue
			}
			pvcsPerNs[pvc.Namespace] = append(pvcsPerNs[pvc.Namespace], pvc.Name)
			fmt.Printf("%s/%s\n", pvc.Namespace, pvc.Name)
		}
	} else {
		pvList, err := clientset.CoreV1().PersistentVolumes().List(metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list persistent volumes: %w", err)
		}
		for _, pv := range pvList.Items {
			if *flags.StorageClass != "" && pv.Spec.StorageClassName != *flags.StorageClass {
				continue
			}
			if pv.Spec.ClaimRef == nil {
				log.Warn("PersistentVolume %s has no ClaimRef, skipping", pv.Name)
				continue
			}
			namespace := pv.Spec.ClaimRef.Namespace
			name := pv.Spec.ClaimRef.Name
			pvcsPerNs[namespace] = append(pvcsPerNs[namespace], name)
			fmt.Printf("%s/%s\n", namespace, name)
		}
	}

	log.Info("Finding pods...")
	var pods []v1.Pod
	for ns, pvcs := range pvcsPerNs {
		podList, err := clientset.CoreV1().Pods(ns).List(metav1.ListOptions{})
		if err != nil {
			return fmt.Errorf("failed to list pods: %w", err)
		}

		for _, pod := range podList.Items {
			if pod.Status.Phase == v1.PodSucceeded || pod.Status.Phase == v1.PodFailed {
				continue
			}
			for _, vol := range pod.Spec.Volumes {
				if vol.PersistentVolumeClaim != nil && slices.Contains(pvcs, vol.PersistentVolumeClaim.ClaimName) {
					fmt.Printf("%s/%s\n", pod.Namespace, pod.Name)
					pods = append(pods, pod)
				}
			}
		}
	}
	log.Info("Found %d pods to scale down", len(pods))

	// TODO: Scale down pods via their associated Deployment/StatefulSet/ReplicaSet

	// TODO: Wait until scaled down. Check for pods in the list until not found anymore (or time out).

	return nil
}
