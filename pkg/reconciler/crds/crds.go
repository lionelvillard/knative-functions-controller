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
	"errors"
	"fmt"

	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	apiextv1beta1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"

	apiextclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	apiextensionsv1beta1 "k8s.io/apiextensions-apiserver/pkg/client/listers/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	servingv1beta1 "knative.dev/serving/pkg/apis/serving/v1beta1"
	servingclient "knative.dev/serving/pkg/client/clientset/versioned"
	servingv1beta1listers "knative.dev/serving/pkg/client/listers/serving/v1beta1"

	"github.com/lionelvillard/knative-functions-controller/pkg/reconciler/crds/resources"
)

// Reconciler implements controller.Reconciler for dynamic resources.
type Reconciler struct {
	// KubeClient allows us to talk to the k8s for core APIs
	kubeClient kubernetes.Interface

	//
	crdClient apiextclientset.Interface

	// servingClient allows us to talk to the serving APIs
	servingClient servingclient.Interface

	// serviceLister index properties about Knative services
	serviceLister servingv1beta1listers.ServiceLister

	// crdLister index properties about CRDs
	crdLister apiextensionsv1beta1.CustomResourceDefinitionLister

	// Recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	Recorder record.EventRecorder
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile implements controller.Reconciler
func (r *Reconciler) Reconcile(ctx context.Context, name string) error {
	logger := logging.FromContext(ctx)

	crd, err := r.crdLister.Get(name)
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logger.Errorf("resource %q no longer exists", name)
		return nil
	} else if err != nil {
		return err
	}

	return r.reconcile(ctx, crd)
}

func (r *Reconciler) reconcile(ctx context.Context, crd *apiextv1beta1.CustomResourceDefinition) error {
	if crd.GetDeletionTimestamp() != nil {
		// Check for a DeletionTimestamp.  If present, elide the normal reconcile logic.
		// When a controller needs finalizer handling, it would go here.
		return nil
	}

	functionName := crd.Spec.Names.Plural

	// Make sure the function service/configmaps exists
	cm, err := r.reconcileConfig(ctx, functionName)
	if err != nil {
		return err
	}

	_, err = r.reconcileService(ctx, functionName, cm)
	if err != nil {

		return err
	}

	return nil
}

func (r *Reconciler) reconcileConfig(ctx context.Context, functionName string) (*corev1.ConfigMap, error) {
	logger := logging.FromContext(ctx)

	cmname := fmt.Sprintf("config-function-%s", functionName)

	cm, err := r.kubeClient.CoreV1().ConfigMaps(system.Namespace()).Get(cmname, metav1.GetOptions{})
	if err != nil {
		if apierrs.IsNotFound(err) {
			cm, err = resources.MakeConfigMap(system.Namespace(), cmname)
			if err != nil {
				logger.Error("Failed to create the function configmap", zap.Error(err))
				return nil, err
			}
			cm, err = r.kubeClient.CoreV1().ConfigMaps(system.Namespace()).Create(cm)
			if err != nil {
				logger.Error("Failed to create the function configmap", zap.Error(err))
				return nil, err
			}
		} else {
			logger.Error("Unable to get the function configmap", zap.Error(err))
			return nil, err
		}
	}

	return cm, nil
}

func (r *Reconciler) reconcileService(ctx context.Context, functionName string, cm *corev1.ConfigMap) (*servingv1beta1.Service, error) {
	logger := logging.FromContext(ctx)

	crd, err := r.crdLister.Get(functionName + ".functions.knative.dev")
	if err != nil {
		logger.Error("Failed to get function Custom Resource Definition", zap.Error(err))
		return nil, fmt.Errorf("Failed to get function Custom Resource Definition: %v", err)
	}

	image, ok := crd.Annotations["functions.knative.dev/image"]
	if !ok {
		logger.Error("Missing functions.knative.dev/image annotation on function CRD", zap.Any("functionName", functionName))
		return nil, errors.New("Missing functions.knative.dev/image annotation on function CRD")
	}

	expected := resources.MakeKnativeService(functionName, cm.ResourceVersion, image)

	// Update service annotation with config map UUID.
	service, err := r.serviceLister.Services("knative-functions").Get(functionName)
	if err != nil {
		if apierrs.IsNotFound(err) {

			ksvc, err := r.servingClient.ServingV1beta1().Services("knative-functions").Create(expected)
			if err != nil {
				logger.Error("Failed to create the function service", zap.Error(err))
				return nil, fmt.Errorf("Failed to create the function service: %v", err)
			}
			return ksvc, nil
		}

		logger.Error("Unable to get the function service", zap.Error(err))
		return nil, err
	} else if !equality.Semantic.DeepEqual(expected.Spec, service.Spec) {
		service = service.DeepCopy()
		service.Spec = expected.Spec

		service, err = r.servingClient.ServingV1beta1().Services("knative-functions").Update(service)
		if err != nil {
			logger.Error("Failed to update the function service", zap.Error(err))
			return nil, fmt.Errorf("Failed to update the function service: %v", err)
		}
	}

	return service, nil
}
