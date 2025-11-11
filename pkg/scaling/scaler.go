package scaling

import (
	"context"
	"fmt"

	"github.com/dancavallaro/kubectl-unmount/pkg/common"
	"github.com/dancavallaro/kubectl-unmount/pkg/logger"
	autoscalingv1 "k8s.io/api/autoscaling/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
)

type Scaler struct {
	clientset *kubernetes.Clientset
	log       *logger.Logger
	dryRun    bool
}

// New creates a new Scaler instance.
func New(clientset *kubernetes.Clientset, log *logger.Logger, dryRun bool) Scaler {
	return Scaler{
		clientset: clientset,
		log:       log,
		dryRun:    dryRun,
	}
}

func (s Scaler) ScaleDown(ctx context.Context, ctrl common.ControllerRef) error {
	if s.dryRun {
		s.log.Info("  (dry-run, skipping controller: %v)", ctrl)
		return nil
	}

	switch ctrl.Kind {
	case common.KindDeployment:
		return scaleControllerToZero(ctx, s.log, s.clientset.AppsV1().Deployments(ctrl.Namespace), ctrl)
	case common.KindStatefulSet:
		return scaleControllerToZero(ctx, s.log, s.clientset.AppsV1().StatefulSets(ctrl.Namespace), ctrl)
	case common.KindReplicaSet:
		return scaleControllerToZero(ctx, s.log, s.clientset.AppsV1().ReplicaSets(ctrl.Namespace), ctrl)
	case common.KindPod:
		return deletePod(ctx, s.log, s.clientset, ctrl)
	case common.KindDaemonSet:
		s.log.Warn("Cannot scale down DaemonSet %s/%s (DaemonSets cannot be scaled)", ctrl.Namespace, ctrl.Name)
		return nil
	default:
		s.log.Warn("Unsupported controller type %s for %s/%s, skipping", ctrl.Kind, ctrl.Namespace, ctrl.Name)
		return nil
	}
}

type scalable interface {
	GetScale(ctx context.Context, deploymentName string, options metav1.GetOptions) (*autoscalingv1.Scale, error)
	UpdateScale(ctx context.Context, deploymentName string, scale *autoscalingv1.Scale, opts metav1.UpdateOptions) (*autoscalingv1.Scale, error)
}

func scaleControllerToZero(ctx context.Context, log *logger.Logger, scaler scalable, ctrl common.ControllerRef) error {
	scale, err := scaler.GetScale(ctx, ctrl.Name, metav1.GetOptions{})
	if err != nil {
		return fmt.Errorf("failed to get scale for %s %s/%s: %w", ctrl.Kind, ctrl.Namespace, ctrl.Name, err)
	}

	originalReplicas := scale.Spec.Replicas
	if originalReplicas == 0 {
		log.Info("%s %s/%s is already scaled to 0", ctrl.Kind, ctrl.Namespace, ctrl.Name)
		return nil
	}

	scale.Spec.Replicas = 0
	_, err = scaler.UpdateScale(ctx, ctrl.Name, scale, metav1.UpdateOptions{})
	if err != nil {
		return fmt.Errorf("failed to scale down %s %s/%s: %w", ctrl.Kind, ctrl.Namespace, ctrl.Name, err)
	}

	log.Info("  Scaled down %s %s/%s from %d to 0 replicas", ctrl.Kind, ctrl.Namespace, ctrl.Name, originalReplicas)
	return nil
}

func deletePod(ctx context.Context, log *logger.Logger, clientset *kubernetes.Clientset, ctrl common.ControllerRef) error {
	err := clientset.CoreV1().Pods(ctrl.Namespace).Delete(ctx, ctrl.Name, metav1.DeleteOptions{})
	if err != nil {
		return fmt.Errorf("failed to delete pod %s/%s: %w", ctrl.Namespace, ctrl.Name, err)
	}
	log.Info("  Deleted standalone Pod %s/%s", ctrl.Namespace, ctrl.Name)
	return nil
}
