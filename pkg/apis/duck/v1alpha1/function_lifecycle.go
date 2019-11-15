/*
Copyright 2019 The Knative Authors.

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

package v1alpha1

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/apis"
	duckv1beta1 "knative.dev/pkg/apis/duck/v1beta1"
)

// GetGroupVersionKind implements kmeta.OwnerRefable
func (fn *Function) GetGroupVersionKind() schema.GroupVersionKind {
	return SchemeGroupVersion.WithKind(fn.Kind)
}

const (
	// FunctionConditionReady has status True when all subconditions below have been set to True.
	FunctionConditionReady = apis.ConditionReady

	// FunctionConditionConfigMapSynced has status true when the function
	// has been synced with the configmap
	FunctionConditionConfigMapSynced apis.ConditionType = "ConfigMapSynced"

	// FunctionConditionAddressable has status true when this Function meets
	// the Addressable contract and has a non-empty URL.
	FunctionConditionAddressable apis.ConditionType = "Addressable"

	// FunctionConditionRouteReady has status true when the route
	// associated to the function is ready
	FunctionConditionRouteReady apis.ConditionType = "RouteReady"

	// FunctionConditionServiceSynced has status true when the function
	// has been synced with the the associated service
	FunctionConditionServiceSynced apis.ConditionType = "ServiceReady"
)

var pFunctionCondSet = apis.NewLivingConditionSet(FunctionConditionReady, FunctionConditionConfigMapSynced, FunctionConditionAddressable)

// GetCondition returns the condition currently associated with the given type, or nil.
func (ps *FunctionStatus) GetCondition(t apis.ConditionType) *apis.Condition {
	return pFunctionCondSet.Manage(ps).GetCondition(t)
}

// IsReady returns true if the resource is ready overall.
func (ps *FunctionStatus) IsReady() bool {
	return pFunctionCondSet.Manage(ps).IsHappy()
}

// InitializeConditions sets relevant unset conditions to Unknown state.
func (ps *FunctionStatus) InitializeConditions() {
	pFunctionCondSet.Manage(ps).InitializeConditions()
}

func (ps *FunctionStatus) MarkConfigMapSynced() {
	pFunctionCondSet.Manage(ps).MarkTrue(FunctionConditionConfigMapSynced)
}

func (ps *FunctionStatus) MarkConfigMapNotSynced(reason, messageFormat string, messageA ...interface{}) {
	pFunctionCondSet.Manage(ps).MarkFalse(FunctionConditionConfigMapSynced, reason, messageFormat, messageA...)
}

func (ps *FunctionStatus) MarkServiceSynced() {
	pFunctionCondSet.Manage(ps).MarkTrue(FunctionConditionServiceSynced)
}

func (ps *FunctionStatus) MarkServiceNotSynced(reason, messageFormat string, messageA ...interface{}) {
	pFunctionCondSet.Manage(ps).MarkFalse(FunctionConditionServiceSynced, reason, messageFormat, messageA...)
}

func (ps *FunctionStatus) MarkRouteReady() {
	pFunctionCondSet.Manage(ps).MarkTrue(FunctionConditionRouteReady)
}

func (ps *FunctionStatus) MarkRouteNotReady(reason, messageFormat string, messageA ...interface{}) {
	pFunctionCondSet.Manage(ps).MarkFalse(FunctionConditionRouteReady, reason, messageFormat, messageA...)
}

func (ps *FunctionStatus) MarkAddressableNotReady(reason, messageFormat string, messageA ...interface{}) {
	pFunctionCondSet.Manage(ps).MarkFalse(FunctionConditionAddressable, reason, messageFormat, messageA...)
}

func (ps *FunctionStatus) SetAddress(url *apis.URL) {
	ps.Address = &duckv1beta1.Addressable{URL: url}

	if url == nil {
		pFunctionCondSet.Manage(ps).MarkFalse(FunctionConditionAddressable, "emptyURL", "URL is the empty string")
		return
	}
	pFunctionCondSet.Manage(ps).MarkTrue(FunctionConditionAddressable)
}
