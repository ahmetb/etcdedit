# etcdedit CLI Interface Design

## Overview

`etcdedit` is a CLI tool for directly editing Kubernetes resources stored in etcd, bypassing the Kubernetes API server.

## Commands

### `etcdedit edit <key>`

Edit a Kubernetes resource at the specified etcd key.

```
etcdedit edit <key> [flags]
```

**Arguments:**
- `<key>` - Full etcd key path (e.g., `/registry/pods/default/my-pod`)

**Flags:**
- `--editor` - Editor command (defaults to `$EDITOR` or `$VISUAL`, falls back to `vi`)
- Etcd connection flags (see Connection Flags section)

**Workflow:**
1. Read value from etcd at specified key
2. Detect encoding (protobuf vs JSON) based on key path
3. Decode to YAML, write to temp file
4. Launch editor
5. On editor exit, attempt to encode YAML back
   - If encoding fails: show error, re-open editor
   - On success: write to etcd
6. Save original value to backup file, display path to user
7. Optimistic concurrency: compare `mod_revision` before and after edit; fail if changed

**Example:**
```
etcdedit edit /registry/pods/default/nginx-pod \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/ca.crt \
  --cert=/etc/kubernetes/pki/etcd/server.crt \
  --key=/etc/kubernetes/pki/etcd/server.key
```

### `etcdedit apply -f <manifest> <key>`

Apply a Kubernetes manifest to a specified etcd key.

```
etcdedit apply -f <manifest.yaml> <key> [flags]
```

**Arguments:**
- `-f, --file` - Path to YAML manifest file
- `<key>` - Full etcd key path

**Flags:**
- Etcd connection flags (see Connection Flags section)

**Workflow:**
1. Read manifest file
2. Determine encoding:
   - If key path matches built-in resource → protobuf
   - Otherwise → JSON (CRD)
3. Encode manifest
4. Write to etcd

**Example:**
```
etcdedit apply -f my-pod.yaml /registry/pods/default/my-pod \
  --endpoints=https://127.0.0.1:2379 \
  --cacert=/etc/kubernetes/pki/etcd/etcd-ca.crt
```

### `etcdedit get <key>`

Decode and print a Kubernetes resource from etcd.

```
etcdedit get <key> [flags]
```

**Arguments:**
- `<key>` - Full etcd key path

**Flags:**
- `-o, --output` - Output format: `yaml` (default) or `json`
- Etcd connection flags (see Connection Flags section)

**Example:**
```
etcdedit get /registry/pods/default/nginx-pod
```

## Connection Flags

Compatible with `etcdctl` flags and environment variables:

| Flag | Environment Variable | Description |
|------|---------------------|-------------|
| `--endpoints` | `ETCDCTL_ENDPOINTS` | Comma-separated etcd endpoints |
| `--cacert` | `ETCDCTL_CACERT` or `ETCDCTL_CA_FILE` | CA certificate file |
| `--cert` | `ETCDCTL_CERT` or `ETCDCTL_CERT_FILE` | Client certificate file |
| `--key` | `ETCDCTL_KEY` or `ETCDCTL_KEY_FILE` | Client key file |
| `--user` | `ETCDCTL_USER` | Username[:password] for authentication |
| `--password` | `ETCDCTL_PASSWORD` | Password for authentication |
| `--insecure-skip-tls-verify` | `ETCDCTL_INSECURE_SKIP_TLS_VERIFY` | Skip TLS verification |
| `--dial-timeout` | - | Dial timeout (default 2s) |

## Key Path Format

Full paths only. The standard Kubernetes etcd key format:

```
/registry/<resource>[/<namespace>]/<name>           # Built-in, namespaced
/registry/<resource>/<name>                         # Built-in, cluster-scoped
/registry/<group>/<resource>/<namespace>/<name>     # CRD, namespaced
/registry/<group>/<resource>/<name>                 # CRD, cluster-scoped
```

**Examples:**
- `/registry/pods/default/my-pod`
- `/registry/services/kube-system/kube-dns`
- `/registry/nodes/worker-1`
- `/registry/clusterroles/admin`
- `/registry/crontabs.stable.example.com/default/my-cron`

## Editor Selection

Editor is determined in this order:
1. `--editor` flag if specified
2. `$EDITOR` environment variable
3. `$VISUAL` environment variable
4. Fallback: `vi`

The editor is launched with the temp file as its argument. The tool waits for the editor process to exit before continuing.

## Backup Behavior

For `edit` command:
- Original etcd value is saved to a temp file before editing
- Temp file location is displayed to user after edit completes
- Temp file is NOT deleted (user can reference it for rollback)
- Location: `$TMPDIR/etcdedit-backup-<key-hash>-<timestamp>.yaml`

## Concurrency Handling

Optimistic concurrency control:
1. Store `mod_revision` when reading from etcd
2. Before writing, fetch current `mod_revision`
3. If changed, fail with error showing conflict
4. User must re-read and re-edit

## Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | General error |
| 2 | Key not found |
| 3 | Editor failed / user aborted |
| 4 | Encoding/decoding error |
| 5 | Connection error |
| 6 | Concurrency conflict (mod_revision changed) |

## Error Handling

### Edit Loop on Encoding Error

If encoding fails after user closes editor (e.g., invalid YAML, type mismatch):
1. Display clear error message
2. Re-open editor with the same temp file
3. User can fix the issue or abort with empty file / quit without saving
4. Loop continues until success or user abort

### User Abort Detection

User aborts editing by:
- Saving an empty file
- The editor exiting with non-zero status

## Implementation Notes

- Use `cobra` for CLI framework
- Use `client-go` etcd client (`go.etcd.io/etcd/client/v3`)
- Use `k8s.io/kubectl/pkg/scheme` for protobuf type registration
- Use `sigs.k8s.io/yaml` for YAML handling
