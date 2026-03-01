# etcdedit

A CLI tool for directly reading, editing, and writing Kubernetes resources in etcd, bypassing the API server entirely.

Kubernetes stores every resource as a protobuf-encoded (or JSON-encoded, for CRDs) value under a key in etcd. `etcdedit` speaks this wire format natively. It decodes resources into human-readable YAML, lets you modify them, and writes them back -- all without the API server ever being involved.

## When to use this

**Cluster recovery.** The API server is crashlooping, admission webhooks are rejecting everything, or a bad RBAC change locked you out. You still have etcd. `etcdedit` lets you fix the broken resource directly and bring the cluster back.

**Surgical edits.** You need to modify a field that the API server validates away, remove a stuck finalizer that a controller keeps re-adding, or patch an object in a way that admission policies forbid.

**Stress testing and benchmarking.** Write thousands of resources into etcd at wire speed without going through admission, validation, or controller reconciliation. Useful for testing how the API server, controllers, and schedulers behave when they discover a sudden surge of pre-existing resources.

**Forensics and debugging.** Export the raw state of any resource exactly as the API server stored it, including internal fields that `kubectl get -o yaml` strips away.

## Install

**Homebrew:**

```bash
brew install ahmetb/tap/etcdedit
```

**Go install:**

```bash
go install github.com/ahmetb/etcdedit@latest
```

**From source:**

```bash
git clone https://github.com/ahmetb/etcdedit.git
cd etcdedit
go build -o etcdedit .
```

## Connecting to etcd

`etcdedit` uses the same flags and environment variables as `etcdctl`. Set them once and every command picks them up:

```bash
export ETCDCTL_ENDPOINTS=https://127.0.0.1:2379
export ETCDCTL_CACERT=/etc/kubernetes/pki/etcd/ca.crt
export ETCDCTL_CERT=/etc/kubernetes/pki/etcd/server.crt
export ETCDCTL_KEY=/etc/kubernetes/pki/etcd/server.key
```

Or pass them as flags:

```bash
etcdedit get /registry/pods/default/nginx \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key
```

The examples below assume the environment variables are set.

## etcd key paths

Every Kubernetes resource lives under a key in etcd. The format is:

```
/registry/<resource>/<namespace>/<name>       # namespaced (pods, configmaps, ...)
/registry/<resource>/<name>                   # cluster-scoped (namespaces, nodes, ...)
/registry/<group>/<resource>/<namespace>/<name>  # CRDs
```

Examples:

```
/registry/pods/default/nginx
/registry/configmaps/kube-system/coredns
/registry/namespaces/default
/registry/clusterroles/admin
/registry/crontabs.stable.example.com/default/my-cron
```

You can discover keys with `etcdctl`:

```bash
etcdctl get /registry/ --prefix --keys-only
```

## Usage

### Export a resource

```bash
etcdedit get /registry/pods/default/nginx
```

This decodes the protobuf (or JSON) value and prints it as YAML. Redirect to a file to save it:

```bash
etcdedit get /registry/pods/default/nginx > nginx.yaml
```

Output as JSON instead:

```bash
etcdedit get /registry/pods/default/nginx -o json
```

### Edit a resource in-place

```bash
etcdedit edit /registry/pods/default/nginx
```

This opens the resource in your `$EDITOR` as YAML. When you save and quit, `etcdedit` encodes it back and writes it to etcd. If the resource was modified by another process while you were editing, the write is rejected and you are asked to retry.

A backup of the original value is saved to a temp file and its path is printed after a successful edit.

To abort, save an empty file or exit the editor with a non-zero status.

### Apply a manifest to a key

```bash
etcdedit apply -f configmap.yaml /registry/configmaps/default/my-config
```

The manifest doesn't need a `metadata.name` -- the name and namespace are derived from the etcd key. If the manifest contains a name that differs from the key, it is overridden automatically (with a warning) so the API server can find the resource.

Built-in resource types are encoded as protobuf. CRDs are encoded as JSON. The encoding is chosen automatically from the key path.

### Clone a resource to a new key

Export, then apply to a different key:

```bash
etcdedit get /registry/configmaps/default/app-config > app-config.yaml
etcdedit apply -f app-config.yaml /registry/configmaps/default/app-config-copy
```

The name and namespace in the manifest are reconciled with the target key automatically.

### Bulk-create resources for stress testing

Write a template, then apply it to many keys in a loop:

```bash
etcdedit get /registry/configmaps/default/template > template.yaml

for i in $(seq 1 1000); do
  etcdedit apply -f template.yaml /registry/configmaps/default/stress-test-$i
done
```

Each resource gets its name from the key, so a single template file works for all of them.

## How it works

Kubernetes stores built-in resource types (pods, services, deployments, etc.) as protobuf using the `k8s.io/apimachinery` serialization format with a 4-byte `k8s\0` magic prefix. Custom resources (CRDs) are stored as plain JSON.

When reading, `etcdedit` detects the encoding from the key path and magic bytes, decodes to a Go typed object (or JSON map for CRDs), and converts to YAML.

When writing, it reverses the process: YAML is converted to an unstructured map, then encoded as protobuf or JSON depending on the key path.

## Limitations

**Protobuf field loss.** When editing protobuf-encoded resources, any fields unknown to the Kubernetes library version that `etcdedit` was compiled against will be silently dropped during the roundtrip. Build against the same or newer version of `k8s.io/api` as your cluster to avoid this.

**No validation.** The API server's admission controllers, validating webhooks, and schema validation are all bypassed. You can write invalid resources that will confuse controllers or the API server itself.

**No controller reconciliation.** Changes are not noticed by controllers until the API server's watch cache picks them up. Some changes may be immediately overwritten by controllers that enforce desired state.

## License

Apache License 2.0 - see [LICENSE](LICENSE) for details.
