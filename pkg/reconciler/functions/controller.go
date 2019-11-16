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

package functions

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	crdinformers "knative.dev/pkg/client/injection/apiextensions/informers/apiextensions/v1beta1/customresourcedefinition"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/clients/dynamicclient"
	"knative.dev/pkg/logging"
	servingclient "knative.dev/serving/pkg/client/injection/client"
	routeinformer "knative.dev/serving/pkg/client/injection/informers/serving/v1beta1/route"
	serviceinformer "knative.dev/serving/pkg/client/injection/informers/serving/v1beta1/service"

	"github.com/lionelvillard/knative-functions-controller/pkg/dynamic"
)

const (
	controllerAgentName = "functions-controller"
)

// NewController returns a new Function reconcile controller.
func NewController(gvr schema.GroupVersionResource) injection.ControllerConstructor {
	return func(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
		logger := logging.FromContext(ctx)

		routeInformer := routeinformer.Get(ctx)
		dynamicInformer := dynamic.Get(ctx, gvr)
		serviceInformer := serviceinformer.Get(ctx)
		crdInformer := crdinformers.Get(ctx)

		c := &Reconciler{
			kubeClient:    kubeclient.Get(ctx),
			dynamicClient: dynamicclient.Get(ctx).Resource(gvr),
			servingClient: servingclient.Get(ctx),
			routeLister:   routeInformer.Lister(),
			serviceLister: serviceInformer.Lister(),
			crdLister:     crdInformer.Lister(),
			Recorder: record.NewBroadcaster().NewRecorder(
				scheme.Scheme, corev1.EventSource{Component: controllerAgentName}),
			functionName: gvr.Resource,
		}
		impl := controller.NewImpl(c, logger, fmt.Sprintf("%s-function", gvr.Resource))

		logger.Info("Setting up event handlers")

		dynamicInformer.Informer().AddEventHandler(controller.HandleAll(impl.Enqueue))

		return impl
	}
}
