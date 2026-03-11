# etcdedit

<p align="center">
  <a href="https://pkg.go.dev/github.com/ahmetb/etcdedit">
    <img src="https://pkg.go.dev/badge/github.com/ahmetb/etcdedit.svg" alt="Go Reference">
  </a>
  <a href="https://github.com/ahmetb/etcdedit/blob/main/LICENSE">
    <img src="https://img.shields.io/github/license/ahmetb/etcdedit" alt="License">
  </a>
</p>

Directly edit Kubernetes resources stored in etcd, bypassing the API server.

> [!WARNING]
> This is a brain surgery tool, use with caution.

This tool directly deals with Kubernetes proto binary encoding (and JSON
encoding for CRDs) so it may cause loss of certain fields depending on which
kubernetes server version you're using.

## Why?

Sometimes you need to directly access the data stored in etcd:

- Recover from a broken cluster state, or restore admin access to the cluster
- Inspect or modify resources when the API server is unavailable
- Restore a resource stuck in an irreversible "deleting" state
- Bypass API validation or webhooks to make a change
- Load the cluster with objects as fast as possible for stress testing

## Installation

```bash
brew tap ahmetb/etcdedit https://github.com/ahmetb/etcdedit
brew install etcdedit
```

```bash
go install github.com/ahmetb/etcdedit@latest
```

## Quick Start

Follow the [kind tutorial](docs/tutorial-kind.md) to try `etcdedit` on a local
cluster in minutes.

## Usage

Set credentials in the env (or --flags) the same way you would for `etcdctl`:

```bash
export ETCDCTL_ENDPOINTS=https://127.0.0.1:2379
export ETCDCTL_CACERT=/etc/kubernetes/pki/etcd/ca.crt
export ETCDCTL_CERT=/etc/kubernetes/pki/etcd/server.crt
export ETCDCTL_KEY=/etc/kubernetes/pki/etcd/server.key
```

- **Get a key**: retrieve a resource from etcd

  ```bash
  etcdedit get /registry/pods/default/nginx
  ```

- **Edit a resource **: launches  $EDITOR (default: vim) to modify a resource
  in place. Before applying the change the original etcd value is saved to a
  temp file.

  ```bash
  etcdedit edit /registry/pods/default/nginx
  ```

- **Apply**: replace the etcd key with a given YAML manifest

  ```bash
  etcdedit apply -f manifest.yaml /registry/pods/default/my-pod
  ```

## etcd Kubernetes Key Paths

You should always run `etcdctl get --keys-only --prefix <PREFIX>` to figure
out which key you should be replacing (**do not guess**).


Most of the key format looks like this:
```text
/registry/<resource>/<namespace>/<name>         # namespaced
/registry/<resource>/<name>                     # cluster-scoped
/registry/<group>/<resource>/<namespace>/<name> # CRD
```
but nothing is guaranteed since this is an implementation detail of the
API server.

## Examples

- Create a ConfigMap (name/namespace derived from key path):

  ```bash
  etcdedit apply -f - /registry/configmaps/default/my-config <<EOF
  apiVersion: v1
  kind: ConfigMap
  metadata:
    labels:
      foo: bar
  data:
    key1: value1
  EOF
  ```

- Restore `cluster-admin` ClusterRoleBinding to a user:

  ```bash
  etcdedit apply -f - /registry/clusterrolebindings/tmp-admin <<EOF
  apiVersion: rbac.authorization.k8s.io/v1
  kind: ClusterRoleBinding
  roleRef:
    apiGroup: rbac.authorization.k8s.io
    kind: ClusterRole
    name: cluster-admin
  subjects:
  - apiGroup: rbac.authorization.k8s.io
    kind: User
    name: ahmet # change your username!
  EOF
  ```

