# Known Issues

Issues discovered through exhaustive testing of etcdedit against a local kind
cluster (Kubernetes v1.35.0) covering `get`, `apply`, and `edit` commands
across 20+ resource types including custom resources.

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

## No delete command

etcdedit provides `get`, `edit`, and `apply` but has no way to delete keys from
etcd. Removing a resource requires falling back to `etcdctl del` or `kubectl
delete`.

This is relevant because one of the stated use cases is "removing resources
stuck in a deletion state" — if a resource has a stuck finalizer, the workflow
would be to `edit` the object to remove the finalizer, but if the goal is to
fully remove the key, users still need a separate tool.
