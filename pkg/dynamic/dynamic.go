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
package dynamic

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/informers"
	"knative.dev/pkg/controller"
	"knative.dev/pkg/logging"

	"github.com/lionelvillard/knative-functions-controller/pkg/dynamic/factory"
)

// Key is used as the key for associating information
// with a context.Context.
type Key struct {
	gvr schema.GroupVersionResource
}

func WithInformer(gvr schema.GroupVersionResource) func(ctx context.Context) (context.Context, controller.Informer) {
	return func(ctx context.Context) (context.Context, controller.Informer) {

		f := factory.Get(ctx)
		inf := f.ForResource(gvr)
		return context.WithValue(ctx, Key{gvr: gvr}, inf), inf.Informer()
	}
}

// Get extracts the Dynamic informer from the context.
func Get(ctx context.Context, gvr schema.GroupVersionResource) informers.GenericInformer {
	untyped := ctx.Value(Key{gvr: gvr})
	if untyped == nil {
		logging.FromContext(ctx).Panicf(
			"Unable to fetch %T from context.", (informers.GenericInformer)(nil))
	}
	return untyped.(informers.GenericInformer)
}
