# Tutorial: Try etcdedit with kind

This tutorial walks you through setting up a local Kubernetes cluster with
[kind](https://kind.sigs.k8s.io/) and using `etcdedit` to directly read and
write resources in etcd.

## Before you begin

Install the following tools:

- [Docker Desktop](https://www.docker.com/products/docker-desktop/)
- [kind](https://kind.sigs.k8s.io/docs/user/quick-start/#installation)
- [kubectl](https://kubernetes.io/docs/tasks/tools/)
- [etcdedit](../README.md#installation)

## Create a kind cluster with etcd exposed

Create a kind cluster that maps the etcd port (2379) to your host:

```bash
cat <<EOF | kind create cluster --name etcdedit-demo --config -
kind: Cluster
apiVersion: kind.x-k8s.io/v1alpha4
nodes:
- role: control-plane
  extraPortMappings:
  - containerPort: 2379
    hostPort: 2379
    protocol: TCP
EOF
```

## Configure etcd credentials

Extract the etcd TLS certificates from the kind node into a local directory:

```bash
mkdir -p /tmp/etcdedit-demo

for f in ca.crt server.crt server.key; do
  docker cp etcdedit-demo-control-plane:/etc/kubernetes/pki/etcd/$f /tmp/etcdedit-demo/$f
done
```

Set the environment variables that `etcdedit` (and `etcdctl`) use:

```bash
export ETCDCTL_ENDPOINTS=https://127.0.0.1:2379
export ETCDCTL_CACERT=/tmp/etcdedit-demo/ca.crt
export ETCDCTL_CERT=/tmp/etcdedit-demo/server.crt
export ETCDCTL_KEY=/tmp/etcdedit-demo/server.key
```

## Verify the connection

Read an existing resource from etcd to confirm everything works:

```bash
etcdedit get /registry/namespaces/default
```

You should see the YAML representation of the `default` namespace.

## Create a resource directly in etcd

Use `etcdedit apply` to write a ConfigMap directly into etcd, bypassing the
API server:

```bash
etcdedit apply -f - /registry/configmaps/default/my-config <<EOF
apiVersion: v1
kind: ConfigMap
metadata:
  labels:
    foo: bar
data:
  greeting: hello world
EOF
```

Verify it exists through the API server:

```bash
kubectl get configmap my-config -o yaml
```

## Edit a resource in etcd

Open the ConfigMap in your editor to modify it in place:

```bash
etcdedit edit /registry/configmaps/default/my-config
```

## Clean up

Delete the kind cluster:

```bash
kind delete cluster --name etcdedit-demo
rm -rf /tmp/etcdedit-demo
```
