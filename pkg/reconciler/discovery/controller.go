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

package discovery

import (
	"context"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	apiextclient "knative.dev/pkg/injection/clients/apiextclient"
	"knative.dev/pkg/injection/clients/kubeclient"
	crdinformers "knative.dev/pkg/injection/informers/apiextinformers/apiextensionsv1beta1/crd"
	"knative.dev/pkg/kmeta"

	"knative.dev/pkg/logging"
)

const (
	controllerAgentName          = "function-discovery-controller"
	dispatcherFunctionAnnotation = "function.knative.dev/dispatcher"
)

// NewController returns a new Function reconcile controller.
func NewController(ctx context.Context, cmw configmap.Watcher) *controller.Impl {
	logger := logging.FromContext(ctx)

	apiextClient := apiextclient.Get(ctx)
	crdInformers := crdinformers.Get(ctx)

	c := &Reconciler{
		kubeClient:   kubeclient.Get(ctx),
		apiextClient: apiextClient,
		Recorder: record.NewBroadcaster().NewRecorder(
			scheme.Scheme, corev1.EventSource{Component: controllerAgentName}),
	}
	impl := controller.NewImpl(c, logger, "discovery")

	logger.Info("Setting up event handlers")

	crdInformers.Informer().AddEventHandler(controller.HandleAll(
		// Filter based on CRD annotation
		func(untyped interface{}) {
			typed, err := kmeta.DeletionHandlingAccessor(untyped)
			if err != nil {
				// TODO: We should consider logging here.
				return
			}

			annotations := typed.GetAnnotations()
			if v, ok := annotations[dispatcherFunctionAnnotation]; ok && v == "true" {
				impl.Enqueue(untyped)
			}
		}))

	return impl
}
