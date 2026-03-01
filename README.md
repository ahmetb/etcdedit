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

## Why etcdedit?

Sometimes you need to directly access the data stored in etcd:

- Recover from a broken cluster state
- Inspect or modify resources when the API server is unavailable
- Debug storage-level issues

## Installation

```bash
brew tap ahmetb/etcdedit https://github.com/ahmetb/etcdedit
brew install etcdedit
```

```bash
go install github.com/ahmetb/etcdedit@latest
```

## Usage

```bash
export ETCDCTL_ENDPOINTS=https://127.0.0.1:2379
export ETCDCTL_CACERT=/etc/kubernetes/pki/etcd/ca.crt
export ETCDCTL_CERT=/etc/kubernetes/pki/etcd/server.crt
export ETCDCTL_KEY=/etc/kubernetes/pki/etcd/server.key
```

### Get a resource

```bash
etcdedit get /registry/pods/default/nginx
```

### Edit a resource

```bash
etcdedit edit /registry/pods/default/nginx
```

### Apply a manifest

```bash
etcdedit apply -f manifest.yaml /registry/pods/default/my-pod
```

## Key Path Format

```text
/registry/<resource>/<namespace>/<name>         # namespaced
/registry/<resource>/<name>                     # cluster-scoped
/registry/<group>/<resource>/<namespace>/<name> # CRD
```

## License

Apache License 2.0
