[![GoDoc](https://godoc.org/github.com/jthomperoo/horizontal-pod-autoscaler?status.svg)](https://godoc.org/github.com/jthomperoo/horizontal-pod-autoscaler)
[![Go Report Card](https://goreportcard.com/badge/github.com/jthomperoo/horizontal-pod-autoscaler)](https://goreportcard.com/report/github.com/jthomperoo/horizontal-pod-autoscaler)
[![License](http://img.shields.io/:license-apache-blue.svg)](http://www.apache.org/licenses/LICENSE-2.0.html)
# Custom Pod Autoscaler - Horizontal Pod Autoscaler (CPA-HPA)

This is the Horizontal Pod Autoscaler (HPA), modified to work as a [Custom Pod Autoscaler (CPA)](https://github.com/jthomperoo/custom-pod-autoscaler). This project is designed to be a starting point to allow developers to quickly take a working Horizontal Pod Autoscaler and modify it as they need.

## What is a Custom Pod Autoscaler?

A Custom Pod Autoscaler is a way to write custom logic into Kubernetes scalers.  
For more detailed overview, see the [Custom Pod Autoscaler Framework](https://github.com/jthomperoo/custom-pod-autoscaler/wiki/Custom-Pod-Autoscaler-Framework).

## How do I use this project?

If you want to deploy this CPA-HPA onto your cluster, you first need to install the [Custom Pod Autoscaler Operator](https://github.com/jthomperoo/custom-pod-autoscaler-operator), follow the [installation guide for instructions for installing the operator](https://github.com/jthomperoo/custom-pod-autoscaler-operator/blob/master/INSTALL.md).

## Overview of codebase

This project should be functionally similar/identical to the Kubernetes Horizontal Pod Autoscaler, with differences being that this is run as a CPA rather than a Kubernetes controller. The code is largely copied from the Kubernetes HPA and modified to work with the Custom Pod Autoscaler. Some restructuring of the architecture was required to fit with how a CPA operates, with the HPA logic now split into two distinct parts; metric gathering and evaluation.

### Metric Gathering

The metric gathering stage takes in a resource to gather metrics for, and metric spec definitions for which metrics to gather. Using this information the metrics are gathered and calculated, from metrics APIs. These metrics are then serialised into JSON and output back to the CPA through stdout.

### Evaluation

The evaluation stage takes in the metrics gathered by the metric gathering stage and makes a decision - how many replicas should the resource have. Once this decision has been made using data available the decision is serialised into JSON and output back to the CPA through stdout to do the actual scaling.

### Configuration

There is some overlap in functionality between the the CPA and HPA, in this project precedence has been given to the CPA functionality. For example, Kubernetes HPAs have the ability to set minimum and maximum replica counts, this project removes those and instead relies on the CPA minimum and maximum replica counts. A Kubernetes HPA also contains a ScaleTargetRef for deciding which resource to target when scaling; for this project only the ScaleTargetRef of the CPA is used.  

Deciding which metrics to use is done by using `MetricSpecs`, which are a key part of HPAs, and look like this:
```yaml
- type: Resource
    resource:
    name: cpu
    target:
        type: Utilization
        averageUtilization: 50
```
To send these specs to the CPA-HPA, add a config option called `metrics` to the CPA, with a multiline string containing the metric list. For example:
```yaml
config: 
  - name: metrics
      value: |
      - type: Resource
          resource:
          name: cpu
          target:
              type: Utilization
              averageUtilization: 50
```

### RBAC permissions

The Custom Pod Autoscaler Operator provisions all required Kubernetes resources, however the `Role` created does not have access to the `metrics.k8s.io` API, in order to allow the CPA-HPA use those metrics the role needs modified to allow this, see the `/example/hpa.yaml` for an example.

## Example

There is an example of how to use the Custom Pod Autoscaler - Horizontal Pod Autoscaler in `/example`. The example has a simple deployment, taken from the [Kubernetes Horizontal Pod Autoscaler walkthrough](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/). The example also contains the YAML defintion of the CPA-HPA.

## Developing this project
### Environment
Developing this project requires these dependencies:

* Go >= 1.13
* Golint
* Docker

### Commands

* `make` - builds the CPA-HPA binary.
* `make docker` - builds the CPA-HPA image.
* `make lint` - lints the code.
* `make vendor` - generates a vendor folder.