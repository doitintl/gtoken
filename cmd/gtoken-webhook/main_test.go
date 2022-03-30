package main

import (
	"context"
	"os"
	"strings"
	"testing"

	cmp "github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	fake "k8s.io/client-go/kubernetes/fake"
)

func TestMain(m *testing.M) {
	testMode = true
	os.Exit(m.Run())
}

//nolint:funlen
func Test_mutatingWebhook_mutateContainers(t *testing.T) {
	type fields struct {
		k8sClient  kubernetes.Interface
		image      string
		pullPolicy string
		volumeName string
		volumePath string
		tokenFile  string
	}
	type args struct {
		containers []corev1.Container
		roleArn    string
		ns         string
	}
	tests := []struct {
		name             string
		fields           fields
		args             args
		mutated          bool
		wantedContainers []corev1.Container
	}{
		{
			name: "mutate single container",
			fields: fields{
				k8sClient:  fake.NewSimpleClientset(),
				volumeName: "test-volume-name",
				volumePath: "/test-volume-path",
				tokenFile:  "test-token",
			},
			args: args{
				containers: []corev1.Container{
					{
						Name:  "TestContainer",
						Image: "test-image",
					},
				},
				roleArn: "arn:aws:iam::123456789012:role/testrole",
				ns:      "test-namespace",
			},
			wantedContainers: []corev1.Container{
				{
					Name:         "TestContainer",
					Image:        "test-image",
					VolumeMounts: []corev1.VolumeMount{{Name: "test-volume-name", MountPath: "/test-volume-path"}},
					Env: []corev1.EnvVar{
						{Name: awsWebIdentityTokenFile, Value: "/test-volume-path/test-token"},
						{Name: awsRoleArn, Value: "arn:aws:iam::123456789012:role/testrole"},
						{Name: awsRoleSessionName, Value: "gtoken-webhook-" + strings.Repeat("0", 16)},
					},
				},
			},
			mutated: true,
		},
		{
			name: "mutate multiple container",
			fields: fields{
				k8sClient:  fake.NewSimpleClientset(),
				volumeName: "test-volume-name",
				volumePath: "/test-volume-path",
				tokenFile:  "test-token",
			},
			args: args{
				containers: []corev1.Container{
					{
						Name:  "TestContainer1",
						Image: "test-image-1",
					},
					{
						Name:  "TestContainer2",
						Image: "test-image-2",
					},
				},
				roleArn: "arn:aws:iam::123456789012:role/testrole",
				ns:      "test-namespace",
			},
			wantedContainers: []corev1.Container{
				{
					Name:         "TestContainer1",
					Image:        "test-image-1",
					VolumeMounts: []corev1.VolumeMount{{Name: "test-volume-name", MountPath: "/test-volume-path"}},
					Env: []corev1.EnvVar{
						{Name: awsWebIdentityTokenFile, Value: "/test-volume-path/test-token"},
						{Name: awsRoleArn, Value: "arn:aws:iam::123456789012:role/testrole"},
						{Name: awsRoleSessionName, Value: "gtoken-webhook-" + strings.Repeat("0", 16)},
					},
				},
				{
					Name:         "TestContainer2",
					Image:        "test-image-2",
					VolumeMounts: []corev1.VolumeMount{{Name: "test-volume-name", MountPath: "/test-volume-path"}},
					Env: []corev1.EnvVar{
						{Name: awsWebIdentityTokenFile, Value: "/test-volume-path/test-token"},
						{Name: awsRoleArn, Value: "arn:aws:iam::123456789012:role/testrole"},
						{Name: awsRoleSessionName, Value: "gtoken-webhook-" + strings.Repeat("0", 16)},
					},
				},
			},
			mutated: true,
		},
		{
			name: "no containers to mutate",
			fields: fields{
				k8sClient:  fake.NewSimpleClientset(),
				volumeName: "test-volume-name",
				volumePath: "/test-volume-path",
			},
			args: args{
				roleArn: "arn:aws:iam::123456789012:role/testrole",
				ns:      "test-namespace",
			},
			mutated: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mw := &mutatingWebhook{
				k8sClient:  tt.fields.k8sClient,
				image:      tt.fields.image,
				pullPolicy: tt.fields.pullPolicy,
				volumeName: tt.fields.volumeName,
				volumePath: tt.fields.volumePath,
				tokenFile:  tt.fields.tokenFile,
			}
			got := mw.mutateContainers(tt.args.containers, tt.args.roleArn)
			if got != tt.mutated {
				t.Errorf("mutatingWebhook.mutateContainers() = %v, want %v", got, tt.mutated)
			}
			if !cmp.Equal(tt.args.containers, tt.wantedContainers) {
				t.Errorf("mutatingWebhook.mutateContainers() = diff %v", cmp.Diff(tt.args.containers, tt.wantedContainers))
			}
		})
	}
}

//nolint:funlen
func Test_mutatingWebhook_mutatePod(t *testing.T) {
	type fields struct {
		image      string
		pullPolicy string
		volumeName string
		volumePath string
		tokenFile  string
	}
	type args struct {
		pod                *corev1.Pod
		ns                 string
		serviceAccountName string
		annotations        map[string]string
		dryRun             bool
	}
	tests := []struct {
		name      string
		fields    fields
		args      args
		wantErr   bool
		wantedPod *corev1.Pod
	}{
		{
			name: "mutate pod",
			fields: fields{
				image:      "doitintl/gtoken:test",
				pullPolicy: "Always",
				volumeName: "test-volume-name",
				volumePath: "/test-volume-path",
				tokenFile:  "test-token",
			},
			args: args{
				pod: &corev1.Pod{
					Spec: corev1.PodSpec{
						Containers: []corev1.Container{
							{
								Name:  "TestContainer",
								Image: "test-image",
							},
						},
						ServiceAccountName: "test-sa",
					},
				},
				ns:                 "test-namespace",
				serviceAccountName: "test-sa",
				annotations:        map[string]string{awsRoleArnKey: "arn:aws:iam::123456789012:role/testrole"},
			},
			wantedPod: &corev1.Pod{
				Spec: corev1.PodSpec{
					InitContainers: []corev1.Container{
						{
							Name:    "generate-gcp-id-token",
							Image:   "doitintl/gtoken:test",
							Command: []string{"/gtoken", "--file=/test-volume-path/test-token", "--refresh=false"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(requestsCPU),
									corev1.ResourceMemory: resource.MustParse(requestsMemory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(limitsCPU),
									corev1.ResourceMemory: resource.MustParse(limitsMemory),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "test-volume-name",
									MountPath: "/test-volume-path",
								},
							},
							ImagePullPolicy: "Always",
						},
					},
					Containers: []corev1.Container{
						{
							Name:         "TestContainer",
							Image:        "test-image",
							VolumeMounts: []corev1.VolumeMount{{Name: "test-volume-name", MountPath: "/test-volume-path"}},
							Env: []corev1.EnvVar{
								{Name: awsWebIdentityTokenFile, Value: "/test-volume-path/test-token"},
								{Name: awsRoleArn, Value: "arn:aws:iam::123456789012:role/testrole"},
								{Name: awsRoleSessionName, Value: "gtoken-webhook-" + strings.Repeat("0", 16)},
							},
						},
						{
							Name:    "update-gcp-id-token",
							Image:   "doitintl/gtoken:test",
							Command: []string{"/gtoken", "--file=/test-volume-path/test-token", "--refresh=true"},
							Resources: corev1.ResourceRequirements{
								Requests: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(requestsCPU),
									corev1.ResourceMemory: resource.MustParse(requestsMemory),
								},
								Limits: corev1.ResourceList{
									corev1.ResourceCPU:    resource.MustParse(limitsCPU),
									corev1.ResourceMemory: resource.MustParse(limitsMemory),
								},
							},
							VolumeMounts: []corev1.VolumeMount{
								{
									Name:      "test-volume-name",
									MountPath: "/test-volume-path",
								},
							},
							ImagePullPolicy: "Always",
						},
					},
					Volumes: []corev1.Volume{
						{
							Name: "test-volume-name",
							VolumeSource: corev1.VolumeSource{
								EmptyDir: &corev1.EmptyDirVolumeSource{
									Medium: corev1.StorageMediumMemory,
								},
							},
						},
					},
					ServiceAccountName: "test-sa",
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sa := &corev1.ServiceAccount{
				ObjectMeta: metav1.ObjectMeta{
					Name:        tt.args.serviceAccountName,
					Namespace:   tt.args.ns,
					Annotations: tt.args.annotations,
				},
			}
			mw := &mutatingWebhook{
				k8sClient:  fake.NewSimpleClientset(sa),
				image:      tt.fields.image,
				pullPolicy: tt.fields.pullPolicy,
				volumeName: tt.fields.volumeName,
				volumePath: tt.fields.volumePath,
				tokenFile:  tt.fields.tokenFile,
			}
			if err := mw.mutatePod(context.TODO(), tt.args.pod, tt.args.ns, tt.args.dryRun); (err != nil) != tt.wantErr {
				t.Errorf("mutatingWebhook.mutatePod() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !cmp.Equal(tt.args.pod, tt.wantedPod) {
				t.Errorf("mutatingWebhook.mutateContainers() = diff %v", cmp.Diff(tt.args.pod, tt.wantedPod))
			}
		})
	}
}
