package common

// ControllerRef represents a Kubernetes controller that owns a pod
type ControllerRef struct {
	Kind      string
	Namespace string
	Name      string
}
