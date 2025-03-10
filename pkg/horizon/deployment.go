/*

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package horizon

import (
	horizonv1 "github.com/openstack-k8s-operators/horizon-operator/api/v1beta1"
	common "github.com/openstack-k8s-operators/lib-common/modules/common"
	"github.com/openstack-k8s-operators/lib-common/modules/common/affinity"
	env "github.com/openstack-k8s-operators/lib-common/modules/common/env"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"
)

const (
	// ServiceCommand is the command used to run Kolla and launch the initial Apache process
	ServiceCommand = "/usr/local/bin/kolla_set_configs && /usr/local/bin/kolla_start"
)

// Deployment creates the k8s deployment structure required to run Horizon
func Deployment(instance *horizonv1.Horizon, configHash string, labels map[string]string) *appsv1.Deployment {
	runAsUser := int64(0)

	args := []string{"-c", ServiceCommand}
	livenessProbe := &corev1.Probe{
		TimeoutSeconds:      15,
		PeriodSeconds:       5,
		InitialDelaySeconds: 15,
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/dashboard/auth/login/?next=/dashboard/",
				Port: intstr.IntOrString{IntVal: 80},
			},
		},
	}
	readinessProbe := &corev1.Probe{
		TimeoutSeconds:      25,
		PeriodSeconds:       5,
		InitialDelaySeconds: 15,
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/dashboard/auth/login/?next=/dashboard/",
				Port: intstr.IntOrString{IntVal: 80},
			},
		},
	}

	startupProbe := &corev1.Probe{
		TimeoutSeconds:      30,
		PeriodSeconds:       60,
		InitialDelaySeconds: 60,
		ProbeHandler: corev1.ProbeHandler{
			HTTPGet: &corev1.HTTPGetAction{
				Path: "/dashboard/auth/login/?next=/dashboard/",
				Port: intstr.IntOrString{IntVal: 80},
			},
		},
	}

	envVars := map[string]env.Setter{}
	envVars["KOLLA_CONFIG_FILE"] = env.SetValue(KollaConfig)
	envVars["KOLLA_CONFIG_STRATEGY"] = env.SetValue("COPY_ALWAYS")
	envVars["ENABLE_DESIGNATE"] = env.SetValue("yes")
	envVars["ENABLE_HEAT"] = env.SetValue("yes")
	envVars["ENABLE_IRONIC"] = env.SetValue("yes")
	envVars["ENABLE_MANILA"] = env.SetValue("yes")
	envVars["ENABLE_OCTAVIA"] = env.SetValue("yes")
	envVars["CONFIG_HASH"] = env.SetValue(configHash)

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceName,
			Namespace: instance.Namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
			Replicas: instance.Spec.Replicas,
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: instance.RbacResourceName(),
					Containers: []corev1.Container{
						{
							Name: ServiceName,
							Command: []string{
								"/bin/bash"},
							Args:  args,
							Image: instance.Spec.ContainerImage,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser: &runAsUser,
							},
							Env:            env.MergeEnvs([]corev1.EnvVar{}, envVars),
							VolumeMounts:   getVolumeMounts(),
							Resources:      instance.Spec.Resources,
							ReadinessProbe: readinessProbe,
							LivenessProbe:  livenessProbe,
							StartupProbe:   startupProbe,
							Ports: []corev1.ContainerPort{
								{
									Name:          "http",
									Protocol:      corev1.ProtocolTCP,
									ContainerPort: 80,
								},
							},
						},
					},
				},
			},
		},
	}
	deployment.Spec.Template.Spec.Volumes = getVolumes(instance.Name)
	deployment.Spec.Template.Spec.Affinity = affinity.DistributePods(
		common.AppSelector,
		[]string{
			ServiceName,
		},
		corev1.LabelHostname,
	)
	if instance.Spec.NodeSelector != nil && len(instance.Spec.NodeSelector) > 0 {
		deployment.Spec.Template.Spec.NodeSelector = instance.Spec.NodeSelector
	}

	return deployment
}
