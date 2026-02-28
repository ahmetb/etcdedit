# Kubernetes Storage Format in etcd

## Overview

Kubernetes stores resources in etcd using two different encodings:

1. **Protobuf** - Built-in Kubernetes types (pods, services, deployments, etc.)
2. **JSON** - Custom Resource Definitions (CRDs) and custom resources

## Key Path Structure

### Built-in Resources

```
/registry/<resource>/<name>                    # cluster-scoped
/registry/<resource>/<namespace>/<name>        # namespaced
```

**Examples:**
- `/registry/pods/default/nginx`
- `/registry/services/default/my-service`
- `/registry/nodes/worker-1`
- `/registry/namespaces/kube-system`
- `/registry/clusterroles/admin`

### Custom Resources (CRDs)

```
/registry/<group>/<resource>/<namespace>/<name>    # namespaced CRD
/registry/<group>/<resource>/<name>                # cluster-scoped CRD
```

**Examples:**
- `/registry/crontabs.stable.example.com/default/my-cron`
- `/registry/clusters.cluster.x-k8s.io/default/my-cluster`

## Encoding Detection Strategy

### For `edit` and `get` commands (reading existing values)

1. Check if key path starts with a known built-in path prefix
2. If yes → attempt protobuf decode
3. If protobuf decode fails → fallback to JSON
4. If no match → attempt JSON decode

### For `apply` command (writing new/updating values)

1. Check if key path matches a built-in resource pattern
2. If yes → encode as protobuf
3. If no → encode as JSON

## Built-in Resource Path Registry

The following paths indicate protobuf-encoded built-in resources:

```go
var builtInPaths = []string{
    "/registry/pods/",
    "/registry/services/",
    "/registry/endpoints/",
    "/registry/configmaps/",
    "/registry/secrets/",
    "/registry/namespaces/",
    "/registry/nodes/",
    "/registry/events/",
    "/registry/limitranges/",
    "/registry/resourcequotas/",
    "/registry/serviceaccounts/",
    "/registry/persistentvolumes/",
    "/registry/persistentvolumeclaims/",
    "/registry/replicationcontrollers/",
    "/registry/deployments/",
    "/registry/replicasets/",
    "/registry/statefulsets/",
    "/registry/daemonsets/",
    "/registry/jobs/",
    "/registry/cronjobs/",
    "/registry/roles/",
    "/registry/rolebindings/",
    "/registry/clusterroles/",
    "/registry/clusterrolebindings/",
    "/registry/storageclasses/",
    "/registry/csistoragecapacities/",
    "/registry/csdrivers/",
    "/registry/csinodes/",
    "/registry/volumeattachments/",
    "/registry/leases/",
    "/registry/priorityclasses/",
    "/registry/runtimeclasses/",
    "/registry/networkpolicies/",
    "/registry/ingresses/",
    "/registry/ingressclasses/",
    "/registry/endpointslices/",
    "/registry/flowschemas/",
    "/registry/prioritylevelconfigurations/",
    "/registry/mutatingwebhookconfigurations/",
    "/registry/validatingwebhookconfigurations/",
    "/registry/customresourcedefinitions/",
    "/registry/controllerrevisions/",
}
```

## Protobuf Encoding Details

### Wire Format

Kubernetes protobuf uses an envelope wrapper format:

```
[4 bytes: magic number "k8s\x00"]
[protobuf-encoded runtime.Unknown message]
```

The `runtime.Unknown` message structure:

```protobuf
message Unknown {
    optional TypeMeta typeMeta = 1;  // apiVersion and kind
    optional bytes raw = 2;          // actual resource protobuf bytes
    optional string contentEncoding = 3;
    optional string contentType = 4;
}
```

### Decode Implementation

```go
import (
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
    "k8s.io/kubectl/pkg/scheme"
)

func decodeProtobuf(data []byte) (runtime.Object, *schema.GroupVersionKind, error) {
    codec := protobuf.NewSerializer(scheme.Scheme, scheme.Scheme)
    
    // First decode to Unknown to get the GVK
    obj := &runtime.Unknown{}
    _, gvk, err := codec.Decode(data, nil, obj)
    if err != nil {
        return nil, nil, err
    }
    
    // Then decode the raw bytes to the concrete type
    intoObj, err := scheme.Scheme.New(*gvk)
    if err != nil {
        return nil, gvk, err
    }
    
    _, _, err = codec.Decode(data, nil, intoObj)
    return intoObj, gvk, err
}
```

### Encode Implementation

```go
func encodeProtobuf(obj runtime.Object) ([]byte, error) {
    codec := protobuf.NewSerializer(scheme.Scheme, scheme.Scheme)
    buf := &bytes.Buffer{}
    err := codec.Encode(obj, buf)
    return buf.Bytes(), err
}
```

### Type Registration

The `k8s.io/kubectl/pkg/scheme` package contains all built-in Kubernetes types pre-registered with their protobuf mappings. This includes:
- Core v1 types (Pod, Service, ConfigMap, etc.)
- Apps v1 types (Deployment, StatefulSet, etc.)
- Batch v1 types (Job, CronJob, etc.)
- RBAC v1 types (Role, ClusterRole, etc.)
- And many more

## JSON Encoding Details (CRDs)

CRD values are stored as raw JSON bytes without any wrapper:

### Decode

```go
import "encoding/json"

func decodeJSON(data []byte) (map[string]interface{}, error) {
    var obj map[string]interface{}
    err := json.Unmarshal(data, &obj)
    return obj, err
}
```

### Encode

```go
func encodeJSON(obj map[string]interface{}) ([]byte, error) {
    return json.Marshal(obj)
}
```

## Roundtrip Flow Diagrams

### Edit Flow (Read → Edit → Write)

```
┌─────────────────────────────────────────────────────────────┐
│                     READ FROM ETCD                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│  Key path matches built-in pattern?                         │
│  ("/registry/pods/", "/registry/services/", etc.)           │
└─────────────────────────────────────────────────────────────┘
           │ Yes                              │ No
           ▼                                  ▼
┌────────────────────────┐      ┌────────────────────────────┐
│  Protobuf decode       │      │  JSON decode               │
│  (using kubectl scheme)│      │  (map[string]interface{})  │
└────────────────────────┘      └────────────────────────────┘
           │                                  │
           └──────────────┬───────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Convert to YAML                                            │
│  (using sigs.k8s.io/yaml)                                   │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Write YAML to temp file                                    │
│  Save mod_revision for optimistic concurrency               │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Launch editor ($EDITOR, $VISUAL, or vi)                    │
│  Wait for editor to exit                                    │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Parse edited YAML                                          │
│  (loop back to editor on parse error)                       │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Was original encoding protobuf?                            │
└─────────────────────────────────────────────────────────────┘
           │ Yes                              │ No
           ▼                                  ▼
┌────────────────────────┐      ┌────────────────────────────┐
│  Convert unstructured  │      │  JSON encode              │
│  to typed Object,      │      │  (json.Marshal)            │
│  protobuf encode       │      └────────────────────────────┘
└────────────────────────┘                  │
           │                                │
           └──────────────┬─────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Check mod_revision (optimistic concurrency)                │
│  Fail if changed since read                                 │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  WRITE TO ETCD                                              │
│  Save original to backup temp file                          │
└─────────────────────────────────────────────────────────────┘
```

### Apply Flow (Manifest → Write)

```
┌─────────────────────────────────────────────────────────────┐
│  Read manifest YAML file                                    │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Parse YAML to map[string]interface{}                       │
└─────────────────────────────────────────────────────────────┘
                          │
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  Key path matches built-in pattern?                         │
└─────────────────────────────────────────────────────────────┘
           │ Yes                              │ No
           ▼                                  ▼
┌────────────────────────┐      ┌────────────────────────────┐
│  Infer GVK from key    │      │  JSON encode              │
│  Convert unstructured  │      │  (CRD)                    │
│  to typed Object,      │      └────────────────────────────┘
│  protobuf encode       │                  │
└────────────────────────┘                  │
           │                                 │
           └──────────────┬──────────────────┘
                          ▼
┌─────────────────────────────────────────────────────────────┐
│  WRITE TO ETCD                                              │
└─────────────────────────────────────────────────────────────┘
```

## Important Limitations

### Unknown Field Loss

**Warning**: When editing protobuf-encoded resources, fields unknown to the compiled version of `etcdedit` will be dropped during the roundtrip.

**Root Cause:**
1. Protobuf decode: raw bytes → typed Go struct
2. Typed Go struct has no fields to hold unknown protobuf fields
3. Protobuf encode: typed Go struct → raw bytes
4. Unknown fields from original are missing in output

**Mitigation**: Compile `etcdedit` against the same or newer version of Kubernetes libraries (`k8s.io/api`, `k8s.io/apimachinery`) as the target cluster.

Example: If your cluster is Kubernetes 1.28, compile `etcdedit` with:
```
go get k8s.io/api@v0.28.0 k8s.io/apimachinery@v0.28.0 k8s.io/kubectl@v0.28.0
```

**Documentation Requirement**: This limitation must be clearly documented in the tool's help output and README.

### resourceVersion Behavior

Note: `resourceVersion` IS stored in etcd values (contrary to some assumptions). It is part of the object metadata and will be visible in the decoded YAML.

When editing:
- The user can modify or remove `resourceVersion`
- etcd will assign a new `resourceVersion` on write regardless
- The `mod_revision` in etcd metadata is separate and used for optimistic concurrency

## etcd Value Metadata

When reading from etcd, the `KeyValue` structure includes:

```go
type KeyValue struct {
    Key            []byte  // e.g., "/registry/pods/default/my-pod"
    CreateRevision int64   // revision when key was created
    ModRevision    int64   // revision of last modification (for concurrency control)
    Version        int64   // version counter (incremented on each update)
    Value          []byte  // the Kubernetes resource (protobuf or JSON)
    Lease          int64   // lease ID if associated with a lease
}
```

For optimistic concurrency:
1. Store `ModRevision` when reading
2. Before writing, optionally check current `ModRevision`
3. If different, another process has modified the key

## Implementation Dependencies

```go
import (
    // Etcd client
    clientv3 "go.etcd.io/etcd/client/v3"
    
    // Kubernetes protobuf support
    "k8s.io/apimachinery/pkg/runtime"
    "k8s.io/apimachinery/pkg/runtime/serializer/protobuf"
    "k8s.io/kubectl/pkg/scheme"
    
    // YAML handling
    "sigs.k8s.io/yaml"
    
    // JSON handling
    "encoding/json"
)
```

## Testing Strategy

1. **Unit tests**: Encoding/decoding roundtrips for known types
2. **Integration tests**: Against a real etcd instance with test data
3. **Compatibility tests**: Edit resources from different Kubernetes versions
