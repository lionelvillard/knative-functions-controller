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

package crds

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	apiextensionsclient "knative.dev/pkg/client/injection/apiextensions/client"
	crdinformers "knative.dev/pkg/client/injection/apiextensions/informers/apiextensions/v1beta1/customresourcedefinition"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	servingclient "knative.dev/serving/pkg/client/injection/client"
	serviceinformer "knative.dev/serving/pkg/client/injection/informers/serving/v1beta1/service"
)

const (
	controllerAgentName = "crd-controller"
)

// NewController returns a new CRD reconcile controller.
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	logger := logging.FromContext(ctx)

	crdInformer := crdinformers.Get(ctx)
	serviceInformer := serviceinformer.Get(ctx)

	r := &Reconciler{
		kubeClient:    kubeclient.Get(ctx),
		crdClient:     apiextensionsclient.Get(ctx),
		crdLister:     crdInformer.Lister(),
		servingClient: servingclient.Get(ctx),
		serviceLister: serviceInformer.Lister(),
		Recorder: record.NewBroadcaster().NewRecorder(
			scheme.Scheme, corev1.EventSource{Component: controllerAgentName}),
	}
	impl := controller.NewImpl(r, logger, "crd")

	logger.Info("Setting up event handlers")

	crdInformer.Informer().AddEventHandler(cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			if object, ok := obj.(metav1.Object); ok {
				if labels := object.GetLabels(); labels != nil {
					if v, ok := labels["functions.knative.dev/crd"]; ok && v == "true" {
						// for _, name := range names {
						// 	if name == object.GetName() {
						// 		return false
						// 	}
						// }
						return true
					}
				}
			}
			return false
		},
		Handler: controller.HandleAll(impl.Enqueue),
	})

	return impl
}
