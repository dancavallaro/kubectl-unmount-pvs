package common

import "fmt"

// ControllerRef represents a Kubernetes controller that owns a pod
type ControllerRef struct {
	Kind      string
	Namespace string
	Name      string
}

func (ref ControllerRef) String() string {
	return fmt.Sprintf("%s/%s/%s", ref.Kind, ref.Namespace, ref.Name)
}
