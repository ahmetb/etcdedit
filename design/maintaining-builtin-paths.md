# Maintaining Built-in Resource Paths

As Kubernetes evolves, new resource types are added. This document describes how to keep `builtInPaths` in sync with the Kubernetes codebase.

## Background

The `builtInPaths` variable in `pkg/codec/codec.go` determines which etcd key prefixes use protobuf encoding vs JSON. When a new built-in resource is added to Kubernetes, it must be added here for etcdedit to correctly decode it.

## Quick Reference

**Source location**: `pkg/kubeapiserver/default_storage_factory_builder.go` in the Kubernetes repository contains `SpecialDefaultResourcePrefixes`.

**Fallback**: The default storage prefix is derived from the lowercase resource name. E.g., `VolumeAttributesClass` → `/registry/volumeattributesclasses/`.

## How to Update

### Option 1: Clone and Check (Recommended)

```bash
# Shallow clone Kubernetes (saves bandwidth)
git clone --depth 1 --filter=blob:none https://github.com/kubernetes/kubernetes.git ~/oss/kubernetes

# List all registry groups
ls -d ~/oss/kubernetes/pkg/registry/*/

# For each group, list resources
for dir in ~/oss/kubernetes/pkg/registry/*/; do
  echo "=== $(basename $dir) ==="
  ls -d "$dir"*/ 2>/dev/null | sed 's|.*/registry/||' | sed 's|/$||'
done
```

### Option 2: Find the Storage Factory Code

The Kubernetes storage factory determines resource prefixes:

1. Open `pkg/kubeapiserver/default_storage_factory_builder.go`
2. Look for `SpecialDefaultResourcePrefixes` map for custom overrides
3. Default prefix is `strings.ToLower(resource)` - so `FooBar` → `/registry/foobar/`

### Option 3: Check API Resources Directly

```bash
# In a running cluster, list all API resources
kubectl api-resources

# Then query etcd to see storage paths
etcdctl get --keys-only --prefix /registry/ | head -100
```

## Key Directories to Monitor

These are the API groups in Kubernetes (check each for new resources):

| Group | Path Pattern |
|-------|--------------|
| core | `/registry/<resource>/` (namespaced or cluster) |
| apps | `/registry/<resource>/` |
| batch | `/registry/<resource>/` |
| networking.k8s.io | `/registry/<resource>/` |
| storage.k8s.io | `/registry/<resource>/` |
| certificates.k8s.io | `/registry/<resource>/` |
| rbac.authorization.k8s.io | `/registry/<resource>/` |
| admissionregistration.k8s.io | `/registry/<resource>/` |
| resource.k8s.io | `/registry/<resource>/` |
| flowcontrol.apiserver.k8s.io | `/registry/<resource>/` |
| coordination.k8s.io | `/registry/<resource>/` |

## Files to Update

When adding new resources, update:

1. **`pkg/codec/codec.go`** - Add to `builtInPaths` slice
2. **`design/storage-format.md`** - Add to the documented path registry

## Common Patterns

### Cluster-scoped resources
- No namespace in path: `/registry/<resource>/<name>`
- Examples: `namespaces`, `nodes`, `clusterroles`, `storageclasses`

### Namespaced resources  
- Include namespace: `/registry/<resource>/<namespace>/<name>`
- Examples: `pods`, `configmaps`, `deployments`

### Special prefixes (from SpecialDefaultResourcePrefixes)
```go
{Group: "", Resource: "replicationcontrollers"}: "controllers"
{Group: "", Resource: "endpoints"}:          "services/endpoints"
{Group: "", Resource: "nodes"}:               "minions"
{Group: "", Resource: "services"}:            "services/specs"
{Group: "extensions", Resource: "ingresses"}: "ingress"
```

### Cohabitating resources
Some resources exist in multiple API groups and share storage:
- `deployments`: extensions + apps
- `daemonsets`: extensions + apps
- `replicasets`: extensions + apps
- `ingresses`: extensions + networking.k8s.io
- `networkpolicies`: extensions + networking.k8s.io

## Verification

After updating, verify with:

```bash
# Test compile
go build ./...

# Test specific resource (requires running cluster)
etcdedit get /registry/<new-resource>/<name>

# Or apply a test resource
etcdedit apply -f - /registry/<new-resource>/test <<EOF
apiVersion: <group>/<version>
kind: <Resource>
metadata:
  name: test
EOF
```

## Example: Adding VolumeAttributesClass

When VolumeAttributesClass was added in Kubernetes 1.31+:

1. Find the resource in registry: `storage/volumeattributesclass/`
2. Determine path: cluster-scoped, lowercase → `/registry/volumeattributesclasses/`
3. Add to `builtInPaths`: `"/registry/volumeattributesclasses/"`
4. Test:
   ```bash
   etcdedit apply -f - /registry/volumeattributesclasses/fast-io <<EOF
   apiVersion: storage.k8s.io/v1
   kind: VolumeAttributesClass
   metadata:
     name: fast-io
   driverName: pd.csi.storage.gke.io
   parameters:
     type: pd-balanced
   EOF
   ```

## Release Checklist

When preparing for a new Kubernetes version:

- [ ] Clone latest Kubernetes or pull updates
- [ ] Compare `pkg/registry/` directory structure against current `builtInPaths`
- [ ] Check for new API groups in `pkg/kubeapiserver/default_storage_factory_builder.go`
- [ ] Update `builtInPaths` in `pkg/codec/codec.go`
- [ ] Update `design/storage-format.md` 
- [ ] Test with at least one new resource type
- [ ] Run `go build ./...` to verify

## Historical Notes

- Kubernetes 1.31+: VolumeAttributesClass, StorageVersionMigration
- Kubernetes 1.30+: ValidatingAdmissionPolicy (GA)
- Kubernetes 1.29+: MutatingAdmissionPolicy (GA)
- Kubernetes 1.28+: ClusterTrustBundle (beta)
