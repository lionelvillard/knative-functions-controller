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
	apiextclient "knative.dev/pkg/client/injection/apiextensions/client"
	crdinformers "knative.dev/pkg/client/injection/apiextensions/informers/apiextensions/v1beta1/customresourcedefinition"
	kubeclient "knative.dev/pkg/client/injection/kube/client"
	"knative.dev/pkg/configmap"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/kmeta"

	"knative.dev/pkg/logging"
)

const (
	controllerAgentName = "functions-discovery-controller"
	functionAnnotation  = "functions.knative.dev/function"
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
			if v, ok := annotations[functionAnnotation]; ok && v == "true" {
				impl.Enqueue(untyped)
			}
		}))

	return impl
}
