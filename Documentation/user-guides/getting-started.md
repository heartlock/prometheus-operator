# Prometheus Operator

Operators were introduced by CoreOS as a class of software that operates other software, putting operational knowledge collected by humans into software. Read more in the [original blog post](https://coreos.com/blog/introducing-operators.html).

The mission of the Prometheus Operator is to make running Prometheus on top of Kubernetes as easy as possible, while preserving configurability as well as making the configuration Kubernetes native.

To follow this getting started you will need a Kubernetes cluster you have access to. Let's give the Prometheus Operator a spin:

[embedmd]:# (../../deployment.yaml)
```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: prometheus-operator
  labels:
    operator: prometheus
spec:
  replicas: 1
  template:
    metadata:
      labels:
        operator: prometheus
    spec:
      containers:
       - name: prometheus-operator
         image: quay.io/coreos/prometheus-operator:v0.7.0
         resources:
           requests:
             cpu: 100m
             memory: 50Mi
           limits:
             cpu: 200m
             memory: 100Mi
```

The Prometheus Operator introduces third party resources in Kubernetes to declare the desired state of a Prometheus and Alertmanager cluster as well as the Prometheus configuration. The resources it introduces are:

* `Prometheus`
* `Alertmanager`
* `ServiceMonitor`

> Important for this guide are the `Prometheus` and `ServiceMonitor` resources. Have a look at the [alerting guide](alerting.md) for more information about the `Alertmanager` resource or the [design doc](../design.md) for an overview of all resources introduced by the Prometheus Operator.

The `Prometheus` resource declaratively describes the desired state of a Prometheus deployment, while a `ServiceMonitor` describes the set of targets to be monitored by Prometheus.

![Prometheus Operator Architecture](images/architecture.png "Prometheus Operator Architecture")

The `Prometheus` resource includes a selection of `ServiceMonitors` to be used, this field is called the `serviceMonitorSelector`.

First, deploy three instances of a simple example application, which listens and exposes metrics on port `8080`.

[embedmd]:# (../../example/user-guides/getting-started/example-app-deployment.yaml)
```yaml
apiVersion: extensions/v1beta1
kind: Deployment
metadata:
  name: example-app
spec:
  replicas: 3
  template:
    metadata:
      labels:
        app: example-app
    spec:
      containers:
      - name: example-app 
        image: fabxc/instrumented_app
        ports:
        - name: web
          containerPort: 8080
```

The `ServiceMonitor` has a label selector to select `Service`s and the underlying `Endpoints` objects. The `Service` object for the example application selects the `Pod`s by the `app` label having the `example-app` value. In addition to that the `Service` object specifies the port the metrics are exposed on.

[embedmd]:# (../../example/user-guides/getting-started/example-app-service.yaml)
```yaml
kind: Service
apiVersion: v1
metadata:
  name: example-app
  labels:
    app: example-app
spec:
  selector:
    app: example-app
  ports:
  - name: web
    port: 8080
```

This `Service` object is discovered by a `ServiceMonitor`, which selects in the same way. The `app` label must have the value `example-app`.

[embedmd]:# (../../example/user-guides/getting-started/example-app-service-monitor.yaml)
```yaml
apiVersion: monitoring.coreos.com/v1alpha1
kind: ServiceMonitor
metadata:
  name: example-app
  labels:
    team: frontend
spec:
  selector:
    matchLabels:
      app: example-app
  endpoints:
  - port: web
```

Finally, a `Prometheus` object defines the `serviceMonitorSelector` to specify which `ServiceMonitor`s should be included. Above the label `team: frontend` was specified, so that's what the `Prometheus` object selects by.

[embedmd]:# (../../example/user-guides/getting-started/prometheus-example.yaml)
```yaml
apiVersion: monitoring.coreos.com/v1alpha1
kind: Prometheus
metadata:
  name: example
spec:
  serviceMonitorSelector:
    matchLabels:
      team: frontend
  resources:
    requests:
      memory: 400Mi
```

This way the frontend team can create new `ServiceMonitor`s and `Service`s resulting in `Prometheus` to be dynamically reconfigured.

To be able to access the Prometheus instance it will have to be exposed to the outside somehow. For demonstration purpose it will be exposed via a `Service` of type `NodePort`.

[embedmd]:# (../../example/user-guides/getting-started/prometheus-example-service.yaml)
```yaml
apiVersion: v1
kind: Service
metadata:
  name: prometheus-example
spec:
  type: NodePort
  ports:
  - name: web
    nodePort: 30900
    port: 9090
    protocol: TCP
    targetPort: web
  selector:
    prometheus: example
```

Once this `Service` is created the Prometheus web UI is available under the node's IP address on port `30900`. The targets page in the web UI now shows that the instances of the example application have successfully been discovered.

> Exposing the Prometheus web UI may not be an applicable solution. Read more about the possibilities of exposing it in the [exposing Prometheus and Alertmanager guide](exposing-prometheus-and-alertmanager.md).

Further reading:

* In addition to managing Prometheus deployments the Prometheus Operator can also manage Alertmanager clusters. Learn more in the [alerting guide](alerting.md).

* Monitoring the Kubernetes cluster itself. Learn more in the [Cluster Monitoring guide](cluster-monitoring.md).
