# Metrics

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

This allows porting over existing Kubernetes HPA metric configurations to the Custom Pod Autoscaler HPA.  
See the [configuration reference for more details](../../reference/configuration#metrics).