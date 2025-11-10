package discovery

import (
	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/logger"
	"k8s.io/client-go/kubernetes"
)

// Finder handles Kubernetes resource discovery operations.
type Finder struct {
	clientset *kubernetes.Clientset
	log       *logger.Logger
}

// New creates a new Finder instance.
func New(clientset *kubernetes.Clientset, log *logger.Logger) Finder {
	return Finder{
		clientset: clientset,
		log:       log,
	}
}
