# Example Custom Pod Autoscaler - Horizontal Pod Autoscaler

This example has a simple deployment, taken from the [Kubernetes Horizontal Pod Autoscaler
walkthrough](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/). The example also
contains the YAML definition of the CPA-HPA.
This example is set up to monitor the deployment defined by `deployment.yaml`; scaling based on CPU usage. If CPU usage
is too high it will scale the deployment up, if it is too low it will scale it down.

## Overview

### Deployment

The deployment is taken from the [Kubernetes Horizontal Pod Autoscaler
walkthrough](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/), it is a simple
web server that responds 'OK!' to HTTP requests.

### Metrics

The metrics are defined in `hpa.yaml` within the `CustomPodAutoscaler` definition:

```yaml
    - name: metrics
      value: |
        - type: Resource
          resource:
            name: cpu
            target:
              type: Utilization
              averageUtilization: 50
```

This metric spec defines targeting CPU utilization, trying to ensure that average CPU utilization will be kept around
50%.

## Usage

To try out this example, follow these steps.

### Prepare the cluster

Prepare the cluster by ensuring that the `metrics-server` is enabled, and installing the [Custom Pod Autoscaler
Operator](https://github.com/jthomperoo/custom-pod-autoscaler-operator/blob/master/INSTALL.md).

1. Use `kubectl apply -f deployment.yaml` to create the deployment to manage, alongside the service exposing it.
2. Use `kubectl apply -f hpa.yaml` to start the autoscaler, pointing at the previously created deployment.

Now your cluster is all set up, with the example application running and the example autoscaler watching it.

3. Use `kubectl run -i --tty load-generator --rm --image=busybox --restart=Never -- /bin/sh -c "while sleep 0.01; do wget -q -O- http://php-apache; done"` to increase the load on the application.

This will increase the load on the application, increasing CPU usage and then causing the autoscaler to scale the
application up.

4. Watch as the number of replicas increases `kubectl get pods`

## More information

See the [wiki for more information, such as guides and references](https://horizontal-pod-autoscaler.readthedocs.io/en/latest/).
