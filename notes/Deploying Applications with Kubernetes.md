# Deploying Applications with Kubernetes

## What is Kubernetes?

An orchestration system for automating deployment, scaling, and operating services running on containers (Docker on steroid).

Pods are smallest deployable unit in Kubernetes. Containers are processes and pods are hosts. 

All containers running in a pod share the same resources like namespace, IP address, volumes, etc.

**Controllers** are control loops that watch the state of resources and make changes when needed.

We use `kind` to run k8s clusters inside Docker containers. Kind runs one Docker container representing one k8s node in the cluster.

## Helm

Package manager to *distribute and install services in k8s*.

Helm packages are called **charts**. A chart defines all resources needed to run a service in a k8s cluster.

A *release* is an instance of running a chart. Each time we install a chart to k8s, Helm creates a release.

*Repositories* are where we share charts and install charts from. 

Some useful commands:

```sh
helm repo add bitnami https://charts.bitnami.com/bitnami

# Make a release
helm install my-nginx bitnami/nginx

helm list

# Confirm nginx running
POD_NAME=$(kubectl get pod \
  --selector=app.kubernetes.io/name=nginx \
  --template '{{index .items 0 "metadata" "name" }}')

SERVICE_IP=$(kubectl get svc \
  --namespace default my-nginx --template "{{ .spec.clusterIP }}")

# New syntax for Helm 4?
kubectl exec $POD_NAME -- curl $SERVICE_IP

# Fix: nginx image does not contain curl to stay lightweight
# so we test by port forward our nginx service
# and use curl from our zsh
kubectl port-forward svc/my-nginx 8080:80
curl localhost:8080
```

Inside a Helm chart:

- `Chart.yaml` describes our chart and `charts` directory may contain subcharts.
- `values.yaml` contains default values like port number, resource requirements, log level, etc.
- `templates` directory contains template files with values to generate valid k8s manifest files.

Generate example chart? with `helm template smolkkafka`

## Deploying our app

Two resource types: `StatefulSet` and `Service`

### `StatefulSet`

`StatefulSet` to manage stateful applications like our service that persists a log, or any service that needs:

- Stable (persisted during scheduling changes), unique network identifiers
- Stable, persistent storage
- Ordered, graceful deployment and scaling
- Ordered, automated rolling updates

> If we are to run a stateless service, use Deployment. One example is running an API with Deployment and Postgres with a StatefulSet.

### `Service`

`Service` exposes an application as a network service. We define a Service with *policies* that specify:

- What Pods the Service applies to
- How to access the Pods

Four types of services specifying how the Service exposes the Pods:

1. `ClusterIP` (default) exposes the Service on a cluster-internal IP so the service is reachable within the k8s cluster.
2. `NodePort` (not recommend) exposes the Service on each Node's IP, even if the Node doesn't have a Pod on it.
3. `LoadBalancer` exposes the Service *externally* using a cloud provider's load balancer. The balancer automatically creates ClusterIP and NodeIP and routes to these services. 
4. `ExternalName` aliases a DNS name.

## Container Probes

k8s use probes to *know if it needs to act on a container* or *to improve our service's reliability*.

Three kinds of probes:

1. Liveness - Container is alive, otherwise k8s will restart the container.
2. Readiness - Container is ready to accept traffic, otherwise k8s will remove the pod from the service load balancers.
3. Startup - Container has started and k8s can begin probing for liveness and readiness

> The systems dedicated to improving the reliability of the service can cause more incidents than the service itself.

Three ways of running probes:

1. Make an HTTP request against a server (Lightweight)
2. Open a TCP socket against a server (Lightweight)
3. Run a command in the container (More precise)
