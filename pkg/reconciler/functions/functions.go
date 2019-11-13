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
	"encoding/json"
	"fmt"
	"strings"

	"github.com/knative/eventing/pkg/utils"
	"go.uber.org/zap"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/equality"
	apierrs "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"knative.dev/pkg/apis"
	"knative.dev/pkg/apis/duck"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"
	"knative.dev/pkg/system"
	"knative.dev/pkg/tracker"
	servingv1beta1 "knative.dev/serving/pkg/apis/serving/v1beta1"
	servingclient "knative.dev/serving/pkg/client/clientset/versioned"
	servingv1beta1listers "knative.dev/serving/pkg/client/listers/serving/v1beta1"

	duckv1alpha1 "github.com/lionelvillard/knative-functions-controller/pkg/apis/duck/v1alpha1"
	"github.com/lionelvillard/knative-functions-controller/pkg/reconciler/functions/resources"
)

// Reconciler implements controller.Reconciler for dynamic resources.
type Reconciler struct {
	// KubeClient allows us to talk to the k8s for core APIs
	kubeClient kubernetes.Interface

	// DynamicClient allows us to talk to the Functions
	dynamicClient dynamic.NamespaceableResourceInterface

	// servingClient allows us to talk to the serving APIs
	servingClient servingclient.Interface

	// RouteLister index properties about Knative routes
	routeLister servingv1beta1listers.RouteLister

	// The tracker builds an index of what resources are watching other
	// resources so that we can immediately react to changes to changes in
	// tracked resources.
	Tracker tracker.Interface

	// Recorder is an event recorder for recording Event resources to the
	// Kubernetes API.
	Recorder record.EventRecorder

	// Function name (eg. filter)
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

	route, err := r.reconcileRoute(ctx, fn)
	if err != nil {
		fn.Status.MarkRouteNotReady("ReconcileFailed", "%v", err)
		return err
	}

	err = r.propagateRouteStatus(ctx, fn, route)
	if err != nil {
		return err
	}

	_, err = r.reconcileConfig(ctx, fn, route)
	if err != nil {
		fn.Status.MarkConfigMapSyncedNotReady("UpdateFailed", "")
		return err
	}
	fn.Status.MarkConfigMapSyncedReady()

	fn.Status.SetAddress(&apis.URL{
		Scheme: "http",
		Host:   fmt.Sprintf("%s.%s.svc.%s", route.Name, route.Namespace, utils.GetClusterDomainName()),
	})

	fn.Status.URL = route.Status.URL
	fn.Status.ObservedGeneration = fn.Generation
	return nil
}

func (r *Reconciler) reconcileRoute(ctx context.Context, fn *duckv1alpha1.Function) (*servingv1beta1.Route, error) {
	logger := logging.FromContext(ctx)

	// Get the  Route and propagate the status to the Function in case it does not exist.
	route, err := r.routeLister.Routes("knative-functions").Get(resources.MakeRouteName(r.functionName, fn.Name, fn.Namespace))
	if err != nil {
		if apierrs.IsNotFound(err) {
			route, err = resources.MakeRoute(r.functionName, fn)
			if err != nil {
				logger.Error("Failed to create the function route object", zap.Error(err))
				return nil, err
			}
			route, err = r.servingClient.ServingV1beta1().Routes("knative-functions").Create(route)
			if err != nil {
				logger.Error("Failed to create the function route", zap.Error(err))
				return nil, err
			}
			return route, nil
		}

		logger.Error("Unable to get the function route", zap.Error(err))
		return nil, err
	}

	// Check to make sure that the Function owns this service and if not, complain.
	if !metav1.IsControlledBy(route, fn) {
		return nil, fmt.Errorf("Function: %s/%s does not own Route: %q", fn.Namespace, fn.Name, route.Name)
	}

	return route, nil
}

func (r *Reconciler) propagateRouteStatus(ctx context.Context, fn *duckv1alpha1.Function, route *servingv1beta1.Route) error {
	c := route.Status.GetCondition(servingv1beta1.RouteConditionReady)
	if c == nil || c.Status != corev1.ConditionTrue {
		if c == nil {
			fn.Status.MarkRouteNotReady("Unknown", "")
		} else {
			fn.Status.MarkRouteNotReady(c.Reason, c.Message)
		}
		return fmt.Errorf("route is not ready")
	}
	fn.Status.MarkRouteReady()
	return nil
}

func (r *Reconciler) reconcileConfig(ctx context.Context, fn *duckv1alpha1.Function, route *servingv1beta1.Route) (*corev1.ConfigMap, error) {
	logger := logging.FromContext(ctx)
	cmname := fmt.Sprintf("config-function-%s", r.functionName)

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

	if _, ok := cm.Data["___config.json"]; !ok {
		cm.Data["___config.json"] = "{}"
	}

	// Deserialize config
	var config map[string]interface{}
	err = json.Unmarshal([]byte(cm.Data["___config.json"]), &config)
	if err != nil {
		logger.Error("Unable to deserialize existing configuration", zap.Error(err))
		return nil, err
	}

	// Update configuration
	// raw, err := json.Marshal(fn.Spec)
	// if err != nil {
	// 	logger.Error("Unable to serialize function instance spec", zap.Error(err))
	// 	return nil, err
	// }
	// data := string(raw)
	data := fn.Spec
	update := false

	key1 := route.Status.Address.URL
	if key1 != nil {
		url1 := strings.TrimLeft(key1.String(), key1.Scheme+"://")
		if old, ok := config[url1]; !ok || !equality.Semantic.DeepEqual(old, data) {
			config[url1] = data
			update = true
		}
	}

	key2 := route.Status.URL
	if key2 != nil {
		url2 := strings.TrimLeft(key2.String(), key2.Scheme+"://")
		if old, ok := config[url2]; !ok || !equality.Semantic.DeepEqual(old, data) {
			config[url2] = data
			update = true
		}
	}

	if update {
		rawconfig, err := json.Marshal(config)
		if err != nil {
			logger.Error("Unable to serialize new configuration", zap.Error(err))
			return nil, err
		}

		cm.Data["___config.json"] = string(rawconfig)
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
