# Knative Eventing Function Controller

This controller manages the lifecycle of Knative Eventing Function instances.

A Knative Eventing Function consists of:
- a CRD defining the function interface, and
- a runtime dispatching and processing Cloud Events.

This controller is responsible for creating the resources needed
to make each function instance _Adressable_ through a shared runtime.


## Getting started

### Prerequites

- A Kubernetes cluster
- [Ko](https://github.com/google/ko)

### Installation

Clone this repository and do:

```sh
ko apply -f config/
kubectl get -n knative-functions deployments.apps
NAME                                        READY   UP-TO-DATE   AVAILABLE   AGE
functions-controller                        1/1     1            1           3d17h
```

## Function Library

The [function library](https://github.com/lionelvillard/knative-functions) contains functions compatible with this controller.
