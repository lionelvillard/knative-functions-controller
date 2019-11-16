/*
Copyright 2019 The Knative Authors

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

package resources

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	servingv1beta1 "knative.dev/serving/pkg/apis/serving/v1beta1"
)

const (
	ConfigMapAnnotation = "functions.knative.dev/cm-resourceVersion"
)

// MakeKnativeService create a knative service
func MakeKnativeService(functionName string, version, image string) *servingv1beta1.Service {
	return &servingv1beta1.Service{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1beta1",
			Kind:       "Service",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      functionName,
			Namespace: "knative-functions",
			Labels: map[string]string{
				FunctionRoleLabel: FunctionRole,
			},
		},
		Spec: servingv1beta1.ServiceSpec{
			ConfigurationSpec: servingv1beta1.ConfigurationSpec{
				Template: servingv1beta1.RevisionTemplateSpec{
					ObjectMeta: metav1.ObjectMeta{
						Annotations: map[string]string{ConfigMapAnnotation: version},
					},
					Spec: servingv1beta1.RevisionSpec{
						PodSpec: corev1.PodSpec{
							Containers: []corev1.Container{
								corev1.Container{
									Image: image,
									VolumeMounts: []corev1.VolumeMount{
										corev1.VolumeMount{
											Name:      "config-function-" + functionName,
											MountPath: "/ko-app/___config.json",
											SubPath:   "___config.json",
										},
									},
								},
							},
							Volumes: []corev1.Volume{
								corev1.Volume{
									Name: "config-function-" + functionName,
									VolumeSource: corev1.VolumeSource{
										ConfigMap: &corev1.ConfigMapVolumeSource{
											LocalObjectReference: corev1.LocalObjectReference{
												Name: "config-function-" + functionName,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}
}
