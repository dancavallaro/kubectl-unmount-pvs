package plugin

import (
	"bytes"
	"context"
	"os"
	"testing"

	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/common"
	"github.com/dancavallaro/kubectl-unmount-pvs/pkg/logger"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/e2e-framework/klient/k8s"
	"sigs.k8s.io/e2e-framework/klient/wait"
	"sigs.k8s.io/e2e-framework/klient/wait/conditions"
	"sigs.k8s.io/e2e-framework/pkg/env"
	"sigs.k8s.io/e2e-framework/pkg/envconf"
	"sigs.k8s.io/e2e-framework/pkg/envfuncs"
	"sigs.k8s.io/e2e-framework/pkg/features"
	"sigs.k8s.io/e2e-framework/support/kind"
)

var (
	storageClassName = "standard"
	testenv          env.Environment
)

func TestMain(m *testing.M) {
	testenv = env.New()
	kindClusterName := envconf.RandomName("test-cluster", 16)

	// Use pre-defined environment funcs to create a kind cluster prior to test run
	testenv.Setup(
		envfuncs.CreateCluster(kind.NewProvider(), kindClusterName),
	)

	testenv.Finish(
		envfuncs.DestroyCluster(kindClusterName),
	)

	os.Exit(testenv.Run(m))
}

func TestRunPlugin(t *testing.T) {
	var logBuf bytes.Buffer
	pluginCfg := &ConfigFlags{
		StorageClass: &storageClassName,
		DryRun:       common.BoolP(false),
		Confirmed:    common.BoolP(true),
		logger:       logger.NewLogger(&logBuf),
	}

	f := features.New("Scale down Deployment").
		Setup(func(ctx context.Context, t *testing.T, config *envconf.Config) context.Context {
			client := config.Client()

			// Create a random namespace
			namespace := envconf.RandomName("test-ns", 16)
			ns := &corev1.Namespace{ObjectMeta: metav1.ObjectMeta{Name: namespace}}
			if err := client.Resources().Create(ctx, ns); err != nil {
				t.Fatal(err)
			}

			// Create a PVC with default StorageClass
			pvc := &corev1.PersistentVolumeClaim{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-pvc",
					Namespace: namespace,
				},
				Spec: corev1.PersistentVolumeClaimSpec{
					StorageClassName: &storageClassName,
					AccessModes: []corev1.PersistentVolumeAccessMode{
						corev1.ReadWriteOnce,
					},
					Resources: corev1.VolumeResourceRequirements{
						Requests: corev1.ResourceList{
							corev1.ResourceStorage: resource.MustParse("1Mi"),
						},
					},
				},
			}
			if err := client.Resources().Create(ctx, pvc); err != nil {
				t.Fatal(err)
			}

			// Create a Pod that uses the PVC
			podSpec := corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "test-container",
						Image: "busybox:latest",
						Command: []string{
							"sh",
							"-c",
							"sleep 3600",
						},
						VolumeMounts: []corev1.VolumeMount{
							{
								Name:      "test-volume",
								MountPath: "/data",
							},
						},
					},
				},
				Volumes: []corev1.Volume{
					{
						Name: "test-volume",
						VolumeSource: corev1.VolumeSource{
							PersistentVolumeClaim: &corev1.PersistentVolumeClaimVolumeSource{
								ClaimName: "test-pvc",
							},
						},
					},
				},
			}
			deployment := &appsv1.Deployment{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-deployment",
					Namespace: namespace,
				},
				Spec: appsv1.DeploymentSpec{
					Replicas: ptr.To[int32](1),
					Selector: &metav1.LabelSelector{
						MatchLabels: map[string]string{
							"app": "test",
						},
					},
					Template: corev1.PodTemplateSpec{
						ObjectMeta: metav1.ObjectMeta{
							Labels: map[string]string{
								"app": "test",
							},
						},
						Spec: podSpec,
					},
				},
			}
			if err := client.Resources().Create(ctx, deployment); err != nil {
				t.Fatal(err)
			}

			err := wait.For(conditions.New(client.Resources()).ResourceMatch(deployment, func(object k8s.Object) bool {
				d := object.(*appsv1.Deployment)
				return d.Status.AvailableReplicas == 1 && d.Status.ReadyReplicas == 1
			}))
			if err != nil {
				t.Error(err)
			}

			pluginCfg.Namespace = &namespace
			return context.WithValue(ctx, "namespace", namespace)
		}).
		Assess("Verify expected Pods are running", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			*pluginCfg.DryRun = true
			logBuf.Reset()
			require.NoError(t, RunPlugin(pluginCfg))
			require.Contains(t, logBuf.String(), "Found 1 pods to scale down")
			require.Contains(t, logBuf.String(), "Found 1 controllers to scale down")
			return ctx
		}).
		Assess("Scale down affected controllers", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			*pluginCfg.DryRun = false
			logBuf.Reset()
			require.NoError(t, RunPlugin(pluginCfg))
			require.Contains(t, logBuf.String(), "Scale down complete")
			return ctx
		}).
		Assess("Verify Pods are no longer running", func(ctx context.Context, t *testing.T, cfg *envconf.Config) context.Context {
			*pluginCfg.DryRun = true
			logBuf.Reset()
			require.NoError(t, RunPlugin(pluginCfg))
			require.Contains(t, logBuf.String(), "No pods found, nothing to do")
			return ctx
		}).
		Feature()

	testenv.Test(t, f)
}
