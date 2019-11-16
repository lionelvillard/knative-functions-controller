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
package main

import (
	"flag"
	"log"
	"os"
	"time"

	"github.com/kelseyhightower/envconfig"
	apiextensionsclientset "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	externalversions "k8s.io/apiextensions-apiserver/pkg/client/informers/externalversions"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/tools/cache"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"
	"knative.dev/pkg/signals"

	"github.com/lionelvillard/knative-functions-controller/pkg/dynamic"
	"github.com/lionelvillard/knative-functions-controller/pkg/reconciler/functions"
)

type envConfig struct {
	Namespace string `envconfig:"SYSTEM_NAMESPACE" default:"default"`
}

var (
	masterURL  = flag.String("master", "", "The address of the Kubernetes API server. Overrides any value in kubeconfig. Only required if out-of-cluster.")
	kubeconfig = flag.String("kubeconfig", "", "Path to a kubeconfig. Only required if out-of-cluster.")
)

func main() {
	flag.Parse()

	cfg, err := sharedmain.GetConfig(*masterURL, *kubeconfig)
	if err != nil {
		log.Fatal("Error building kubeconfig", err)
	}

	clientset := apiextensionsclientset.NewForConfigOrDie(cfg)
	defs, err := clientset.ApiextensionsV1beta1().CustomResourceDefinitions().List(metav1.ListOptions{
		LabelSelector: "functions.knative.dev/crd=true",
	})

	if err != nil {
		log.Fatal("Error listing custom resource definitions", err)
	}

	var env envConfig
	err = envconfig.Process("", &env)
	if err != nil {
		log.Fatalf("Error processing environment: %v", err)
	}

	ctx := signals.NewContext()

	controllers := make([]injection.ControllerConstructor, len(defs.Items))
	names := make([]string, len(defs.Items))
	for i, crd := range defs.Items {
		gvr := schema.GroupVersionResource{
			Group:    crd.Spec.Group,
			Version:  crd.Spec.Version,
			Resource: crd.Spec.Names.Plural,
		}
		names[i] = crd.Name

		log.Printf("injecting %v", gvr)

		injection.Default.RegisterInformer(dynamic.WithInformer(gvr))

		controllers[i] = functions.NewController(gvr)
	}

	f := externalversions.NewSharedInformerFactory(clientset, time.Hour)
	crdInformer := f.Apiextensions().V1beta1().CustomResourceDefinitions().Informer()
	crdInformer.AddEventHandler(newHandler(names))
	go crdInformer.Run(ctx.Done())

	if len(controllers) > 0 {
		sharedmain.MainWithConfig(ctx, "controller", cfg, controllers...)
	}

	<-ctx.Done()
}

func newHandler(names []string) cache.ResourceEventHandler {
	return cache.FilteringResourceEventHandler{
		FilterFunc: func(obj interface{}) bool {
			if object, ok := obj.(metav1.Object); ok {
				if labels := object.GetLabels(); labels != nil {
					if v, ok := labels["functions.knative.dev/crd"]; ok && v == "true" {
						for _, name := range names {
							if name == object.GetName() {
								return false
							}
						}

						return true
					}
				}
			}
			return false
		},
		Handler: restartResourceHandler{}}
}

type restartResourceHandler struct{}

func (r restartResourceHandler) OnAdd(obj interface{}) {
	// TODO: close channel
	os.Exit(0)
}

func (r restartResourceHandler) OnUpdate(oldObj, newObj interface{}) {
	os.Exit(0)
}

func (r restartResourceHandler) OnDelete(obj interface{}) {
	os.Exit(0)
}
