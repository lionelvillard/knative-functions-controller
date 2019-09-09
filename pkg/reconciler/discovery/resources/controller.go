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
	servingv1beta1 "knative.dev/serving/pkg/apis/serving/v1beta1"
)

// MakeFunctionController creates a new Function controller
func MakeFunctionController(namespace, name string) (*servingv1beta1.Service, error) {
	// // Add annotations
	// service := &servingv1beta1.Service{
	// 	TypeMeta: metav1.TypeMeta{
	// 		APIVersion: "v1",
	// 		Kind:       "ConfigMap",
	// 	},
	// 	ObjectMeta: metav1.ObjectMeta{
	// 		Name:      name,
	// 		Namespace: namespace,
	// 		// OwnerReferences: []metav1.OwnerReference{
	// 		// 	*kmeta.NewControllerRef(fn),
	// 		// },
	// 	},
	// 	Data: map[string]string{},
	// }

	// return cm, nil
	return nil, nil
}
