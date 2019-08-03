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

package reconciler

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/knative/eventing/pkg/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	corev1listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	"knative.dev/pkg/tracker"

	duckv1alpha1 "github.com/lionelvillard/knative-functions-controller/pkg/apis/duck/v1alpha1"
	"github.com/lionelvillard/knative-functions-controller/pkg/reconciler/resources"
)

// Reconciler implements controller.Reconciler for dynamic resources.
type Reconciler struct {
	// KubeClient allows us to talk to the k8s for core APIs
	kubeClient kubernetes.Interface

	// DynamicClient allows us to talk to the Functions
	dynamicClient dynamic.NamespaceableResourceInterface

	// Listers index properties about services
	ServiceLister corev1listers.ServiceLister

	// The tracker builds an index of what resources are watching other
	// resources so that we can immediately react to changes to changes in
	// tracked resources.
	Tracker tracker.Interface

	// Recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	Recorder record.EventRecorder

	// Function name
	functionName string
}

// Check that our Reconciler implements controller.Reconciler
var _ controller.Reconciler = (*Reconciler)(nil)

// Reconcile implements controller.Reconciler
func (r *Reconciler) Reconcile(ctx context.Context, key string) error {
	logger := logging.FromContext(ctx)

	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		logger.Errorf("invalid resource key: %s", key)
		return nil
	}

	untyped, err := r.dynamicClient.Namespace(namespace).Get(name, metav1.GetOptions{})
	if apierrs.IsNotFound(err) {
		// The resource may no longer exist, in which case we stop processing.
		logger.Errorf("resource %q no longer exists", key)
		return nil
	} else if err != nil {
		return err
	}

	original := &duckv1alpha1.Function{}
	duck.FromUnstructured(untyped, original)

	if _, ok := original.Spec["language"]; !ok {
		logger.Error("missing language property")
		return nil
	}

	resource := original.DeepCopy()

	// Reconcile this copy of the resource and then write back any status
	// updates regardless of whether the reconciliation errored out.
	reconcileErr := r.reconcile(ctx, resource)
	if equality.Semantic.DeepEqual(original.Status, resource.Status) {
		// If we didn't change anything then don't call updateStatus.
		// This is important because the copy we loaded from the informer's
		// cache may be stale and we don't want to overwrite a prior update
		// to status with this stale state.
	} else if _, err = r.updateStatus(resource); err != nil {
		logger.Warnw("Failed to update resource status", zap.Error(err))
		r.Recorder.Eventf(resource, corev1.EventTypeWarning, "UpdateFailed",
			"Failed to update status for %q: %v", resource.Name, err)
		return err
	}
	if reconcileErr != nil {
		r.Recorder.Event(resource, corev1.EventTypeWarning, "InternalError", reconcileErr.Error())
	}
	return reconcileErr
}

func (r *Reconciler) reconcile(ctx context.Context, fn *duckv1alpha1.Function) error {
	//logger := logging.FromContext(ctx)
	if fn.GetDeletionTimestamp() != nil {
		// Check for a DeletionTimestamp.  If present, elide the normal reconcile logic.
		// When a controller needs finalizer handling, it would go here.
		return nil
	}

	fn.Status.InitializeConditions()

	svc, err := r.reconcileFunctionService(ctx, fn)
	if err != nil {
		return err
	}

	_, err = r.reconcileConfig(ctx, fn, svc)
	if err != nil {
		fn.Status.MarkConfigMapSyncedNotReady("UpdateFailed", "")
		return err
	}
	fn.Status.MarkConfigMapSyncedReady()

	fn.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.%s", svc.Name, svc.Namespace, utils.GetClusterDomainName()),
	})

	fn.Status.ObservedGeneration = fn.Generation
	return nil
}

func (r *Reconciler) reconcileFunctionService(ctx context.Context, fn *duckv1alpha1.Function) (*corev1.Service, error) {
	logger := logging.FromContext(ctx)
	lang := fn.Spec["language"].(string)
	// Get the  Service and propagate the status to the Function in case it does not exist.
	// We don't do anything with the service because it's status contains nothing useful, so just do
	// an existence check. Then below we check the endpoints targeting it.
	// We may change this name later, so we have to ensure we use proper addressable when resolving these.
	svc, err := r.ServiceLister.Services(fn.Namespace).Get(resources.MakeFunctionServiceName(fn.Name))
	if err != nil {
		if apierrs.IsNotFound(err) {
			svc, err = resources.MakeK8sService(fn, resources.ExternalService(system.Namespace(), r.functionName, lang))
			if err != nil {
				logger.Error("Failed to create the function service object", zap.Error(err))
				return nil, err
			}
			svc, err = r.kubeClient.CoreV1().Services(fn.Namespace).Create(svc)
			if err != nil {
				logger.Error("Failed to create the function service", zap.Error(err))
				return nil, err
			}
			return svc, nil
		}

		logger.Error("Unable to get the function service", zap.Error(err))
		return nil, err
	}
	// Check to make sure that the Function owns this service and if not, complain.
	if !metav1.IsControlledBy(svc, fn) {
		return nil, fmt.Errorf("Function: %s/%s does not own Service: %q", fn.Namespace, fn.Name, svc.Name)
	}
	return svc, nil
}

func (r *Reconciler) reconcileConfig(ctx context.Context, fn *duckv1alpha1.Function, svc *corev1.Service) (*corev1.ConfigMap, error) {
	logger := logging.FromContext(ctx)
	lang := fn.Spec["language"].(string)
	cmname := fmt.Sprintf("%s-%s", r.functionName, lang)

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
	if cm.Data == nil {
		cm.Data = make(map[string]string)
	}
	raw, err := json.Marshal(fn.Spec)
	if err != nil {
		logger.Error("Unable to serialize function instance spec", zap.Error(err))
		return nil, err
	}
	data := string(raw)
	key := fmt.Sprintf("%s.%s.svc.%s", svc.Name, svc.Namespace, utils.GetClusterDomainName())
	if old, ok := cm.Data[key]; !ok || data != old {
		cm.Data[key] = string(data)
		return r.kubeClient.CoreV1().ConfigMaps(system.Namespace()).Update(cm)
	}

	return cm, nil
}

// Update the Status of the resource.  Caller is responsible for checking
// for semantic differences before calling.
func (r *Reconciler) updateStatus(desired *duckv1alpha1.Function) (*unstructured.Unstructured, error) {
	// Use the unstructured marshaller to ensure it's proper JSON
	raw, err := json.Marshal(desired)
	if err != nil {
		return nil, err
	}

	object := unstructured.Unstructured{}
	object.UnmarshalJSON(raw)

	return r.dynamicClient.Namespace(desired.Namespace).UpdateStatus(&object, metav1.UpdateOptions{})
}
