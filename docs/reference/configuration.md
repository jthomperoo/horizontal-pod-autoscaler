# Configuration

You can configure

## How to provide configuration

Configuration is passed as environment variables, defined in the `CustomPodAutoscaler` YAML. This allows modifying these values at deploy time, and overriding defaults.   
For example:
```yaml
apiVersion: custompodautoscaler.com/v1alpha1
kind: CustomPodAutoscaler
metadata:
  name: horizontal-pod-autoscaler-example
spec:
  template:
    spec:
      containers:
      - name: horizontal-pod-autoscaler-example
        image: horizontal-pod-autoscaler:latest
        imagePullPolicy: Always
  scaleTargetRef:
    apiVersion: apps/v1
    kind: Deployment
    name: php-apache
  config: 
    - name: minReplicas
      value: "1"
    - name: maxReplicas
      value: "3"
    - name: interval
      value: "30000"
    - name: tolerance
      value: "0.2"
    - name: downscaleStabilization
      value: "120"
    - name: metrics
      value: |
        - type: Resource
          resource:
            name: cpu
            target:
              type: Utilization
              averageUtilization: 50
```

## cpuInitializationPeriod

Example:
```yaml
    - name: cpuInitializationPeriod
      value: "150"
```
Default value: `300` (5 minutes).  
Set in seconds.  
Equivalent to `--horizontal-pod-autoscaler-cpu-initialization-period`; the period after pod start when CPU samples might be skipped.  

## initialReadinessDelay

Example:
```yaml
    - name: initialReadinessDelay
      value: "45"
```
Default value: `30` (30 seconds).  
Set in seconds.  
Equivalent to `--horizontal-pod-autoscaler-initial-readiness-delay`; the period after pod start during which readiness changes will be treated as initial readiness.

## tolerance

Example:
```yaml
    - name: tolerance
      value: "0.2"
```
Default value: `0.1`.  
Equivalent to `--horizontal-pod-autoscaler-tolerance`; the minimum change (from 1.0) in the desired-to-actual metrics ratio for the horizontal pod autoscaler to consider scaling.

## metrics

Example:
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
Equivalent to K8s HPA metric specs; which are [demonstrated in this HPA walkthrough](https://kubernetes.io/docs/tasks/run-application/horizontal-pod-autoscale-walkthrough/#autoscaling-on-multiple-metrics-and-custom-metrics).  
Can hold multiple values as it is an array.