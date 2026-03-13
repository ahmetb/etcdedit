# Known Issues

Issues discovered through exhaustive testing of etcdedit against a local kind
cluster (Kubernetes v1.35.0) covering `get`, `apply`, and `edit` commands
across 20+ resource types including custom resources.

---

## `--dial-timeout` flag is not honored

When connecting to an unreachable etcd endpoint, the tool hangs indefinitely
regardless of the `--dial-timeout` value. The timeout is never enforced, and
the process must be killed externally (e.g. via `kill` or `Ctrl-C`).

This makes it impossible to use etcdedit in scripts or automation where a
prompt failure is expected when etcd is unavailable.

```bash
# Expected: fail after 1 second with a connection error
# Actual: hangs forever
etcdedit get /registry/namespaces/default \
  --endpoints https://127.0.0.1:9999 \
  --dial-timeout 1s
```

---

## Node objects cannot be decoded

Attempting to `get` a Node resource from etcd fails with a JSON parse error.
Kubernetes stores Node objects (under `/registry/minions/`) using protobuf
serialization rather than JSON. etcdedit can only decode JSON-encoded resources,
so any protobuf-only resource type will fail.

This is notable because Nodes are a common resource users would want to inspect
or modify directly in etcd (e.g. to fix a cordoned node when the API server is
down).

```bash
etcdedit get /registry/minions/my-node
# Error: invalid character 'k' looking for beginning of value
```

It's worth investigating whether other core resource types also use protobuf
storage in newer Kubernetes versions, as this could affect the tool's usefulness
over time.

---

## `-f -` (stdin) is not recognized by apply

The conventional Unix idiom of using `-` to mean "read from stdin" does not
work with the `-f` flag. Instead of reading from standard input, the tool
attempts to open a literal file named `-`, which fails.

This breaks the pattern shown in the tutorial (`etcdedit apply -f - <key>
<<EOF`) and makes piping manifests inconvenient. The workaround is to use
`-f /dev/stdin`, which works but is non-obvious and differs from the convention
used by `kubectl apply -f -`.

```bash
# Fails — tries to open file literally named "-"
cat manifest.yaml | etcdedit apply -f - /registry/configmaps/default/test

# Workaround
cat manifest.yaml | etcdedit apply -f /dev/stdin /registry/configmaps/default/test
```

---

## Service key path causes incorrect namespace inference

Kubernetes stores Service objects at the etcd path
`/registry/services/specs/<namespace>/<name>`, which has an extra `specs`
segment compared to the standard `/registry/<resource>/<namespace>/<name>`
pattern used by all other namespaced resources.

When applying a Service, etcdedit appears to infer the namespace from the key
path and misidentifies `specs` as the namespace, overriding whatever is set in
`metadata.namespace`. This results in a Service object whose namespace metadata
is wrong.

```bash
etcdedit apply -f /dev/stdin /registry/services/specs/test-ns/my-svc <<'EOF'
apiVersion: v1
kind: Service
metadata:
  name: my-svc
  namespace: test-ns
spec:
  ports:
  - port: 80
EOF
# metadata.namespace gets set to "specs" instead of "test-ns"
```

The same path structure applies to Endpoints
(`/registry/services/endpoints/<namespace>/<name>`), which may also be
affected.

---

## `--editor` flag does not handle shell quoting correctly

The `--editor` flag splits its value on whitespace rather than invoking it
through a shell. This means any editor command containing spaces, quotes, or
shell metacharacters will be parsed incorrectly. For example, a `sed` command
with a substitution pattern gets split into multiple arguments, and the quotes
around the pattern are passed literally rather than being interpreted by a
shell.

This significantly limits the usefulness of non-interactive editing, since most
real-world editor commands involve arguments with spaces.

```bash
# Fails — arguments are split incorrectly
etcdedit edit /registry/configmaps/default/test \
  --editor "sed -i.bak 's/old/new/'"

# Workaround — use a wrapper script
cat > /tmp/edit.sh <<'SCRIPT'
#!/bin/sh
sed -i.bak 's/old/new/' "$1"
SCRIPT
chmod +x /tmp/edit.sh
etcdedit edit /registry/configmaps/default/test --editor /tmp/edit.sh
```

The fix would be to pass the `--editor` value through `sh -c` (with the temp
file path appended) rather than splitting on whitespace.

---

## No-op edits still write back to etcd

When using `edit`, if the editor exits without modifying the file, etcdedit
still writes the unchanged object back to etcd. This increments the resource's
`resourceVersion` and triggers any watches, even though nothing actually
changed.

This is wasteful and can be confusing when auditing changes. Tools like
`kubectl edit` detect when no changes were made and skip the write. etcdedit
should compare the pre-edit and post-edit content and only write back if
something actually differs.

---

## Multi-document YAML is silently truncated

When a YAML file containing multiple documents (separated by `---`) is passed
to `apply`, only the first document is applied. All subsequent documents are
silently discarded with no warning or error, and the command exits with code 0.

This is a data loss hazard. Users who assemble manifests with `---` separators
(a common Kubernetes convention) will unknowingly lose everything after the
first document.

```bash
# Only doc1 is applied; doc2 is silently ignored
etcdedit apply -f /dev/stdin /registry/configmaps/default/test <<'EOF'
apiVersion: v1
kind: ConfigMap
metadata:
  name: doc1
data:
  key1: value1
---
apiVersion: v1
kind: ConfigMap
metadata:
  name: doc2
data:
  key2: value2
EOF
```

The tool should either reject multi-document input with a clear error, or at
minimum emit a warning that extra documents were ignored.

---

## No delete command

etcdedit provides `get`, `edit`, and `apply` but has no way to delete keys from
etcd. Removing a resource requires falling back to `etcdctl del` or `kubectl
delete`.

This is relevant because one of the stated use cases is "removing resources
stuck in a deletion state" — if a resource has a stuck finalizer, the workflow
would be to `edit` the object to remove the finalizer, but if the goal is to
fully remove the key, users still need a separate tool.
