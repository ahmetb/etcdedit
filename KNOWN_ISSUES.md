# Known Issues


## No-op edits still write back to etcd

When using `edit`, if the editor exits without modifying the file, etcdedit
still writes the unchanged object back to etcd. This increments the resource's
`resourceVersion` and triggers any watches, even though nothing actually
changed.

This is wasteful and can be confusing when auditing changes. Tools like
`kubectl edit` detect when no changes were made and skip the write. etcdedit
should compare the pre-edit and post-edit content and only write back if
something actually differs.


## Some metadata fields are incomplete during `apply`

The tool doesn't add `creationTimestamp` for example. But you should be able
to manually enter these bits of information fairly easily.
