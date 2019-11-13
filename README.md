# Knative Eventing Function Controller

This controller manages the lifecycle of Knative Eventing Function instances.

A Knative Eventing Function consists of:
- a CRD defining the function interface, and
- a runtime dispatching and processing Cloud Events.

This controller is responsible for creating the resources needed
to make each function instance _Adressable_ through a shared runtime.

## Getting started

### Prerequisites

- A Kubernetes cluster

### Installation

Run:

```sh
kubectl apply -f https://github.com/lionelvillard/knative-functions-controller/releases/download/v0.1.0/function.yaml
```

Check the controller is ready:

```sh
kubectl get -n knative-functions deployments.apps
NAME                                        READY   UP-TO-DATE   AVAILABLE   AGE
functions-controller                        1/1     1            1           3d17h
```

## Function Library

The [function library](https://github.com/lionelvillard/knative-functions) contains functions compatible with this controller.
