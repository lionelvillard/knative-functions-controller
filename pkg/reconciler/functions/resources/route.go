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
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"knative.dev/pkg/kmeta"
	servingv1beta1 "knative.dev/serving/pkg/apis/serving/v1beta1"

	duckv1alpha1 "github.com/lionelvillard/knative-functions-controller/pkg/apis/duck/v1alpha1"
)

const (
	portName          = "endpoint"
	portNumber        = 80
	FunctionRoleLabel = "function.knative.dev/role"
	FunctionRole      = "dispatcher"
)

// RouteOption can be used to optionally modify the Route in MakeRoute.
type RouteOption func(*servingv1beta1.Route) error

// func MakeExternalServiceAddress(namespace, service, language string) string {
// 	return fmt.Sprintf("%s-%s.%s.svc.%s", service, language, namespace, utils.GetClusterDomainName())
// }

func MakeRouteName(functionName, name, ns string) string {
	return fmt.Sprintf("%s-%s-%s", functionName, ns, name)
}

func MakeRoute(functionName string, fn *duckv1alpha1.Function, opts ...RouteOption) (*servingv1beta1.Route, error) {
	// Add annotations
	tr := true
	lang := fn.Spec["language"].(string)
	route := &servingv1beta1.Route{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "v1beta1",
			Kind:       "Route",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      MakeRouteName(functionName, fn.Name, fn.Namespace),
			Namespace: "knative-functions",
			Labels: map[string]string{
				FunctionRoleLabel: FunctionRole,
			},
			OwnerReferences: []metav1.OwnerReference{
				*kmeta.NewControllerRef(fn),
			},
		},
		Spec: servingv1beta1.RouteSpec{
			Traffic: []servingv1beta1.TrafficTarget{
				{
					ConfigurationName: fmt.Sprintf("%s-dispatcher-%s", functionName, lang),
					LatestRevision:    &tr,
					Percent:           100,
				},
			},
		},
	}
	for _, opt := range opts {
		if err := opt(route); err != nil {
			return nil, err
		}
	}
	return route, nil
}
