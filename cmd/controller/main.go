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
	"github.com/kelseyhightower/envconfig"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"knative.dev/pkg/injection"
	"knative.dev/pkg/injection/sharedmain"

	"github.com/lionelvillard/knative-functions-controller/pkg/dynamic"
	"github.com/lionelvillard/knative-functions-controller/pkg/reconciler/functions"
)

type envConfig struct {
	Namespace string   `envconfig:"SYSTEM_NAMESPACE" default:"default"`
	GVR       []string `envconfig:"GVR" required:"true"`
}

func main() {
	logCfg := zap.NewProductionConfig()
	logCfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	dlogger, err := logCfg.Build()
	logger := dlogger.Sugar()

	var env envConfig
	err = envconfig.Process("", &env)
	if err != nil {
		logger.Fatalw("Error processing environment", zap.Error(err))
	}

	controllers := make([]injection.ControllerConstructor, len(env.GVR))
	for i, raw := range env.GVR {
		gvr, _ := schema.ParseResourceArg(raw)
		logger.Infof("injecting %v", *gvr)

		injection.Default.RegisterInformer(dynamic.WithInformer(*gvr))

		controllers[i] = functions.NewController(*gvr)
	}

	sharedmain.Main("controller", controllers...)
}
