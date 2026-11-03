# watchedby

Use `watchedby` when a change to one resource should wake dependent resources.
The notifier lists the dependents and calls `Update` on them. Your storage
backend must deliver those updates to the dependent control loop.

The [Alert → Messenger example](../../examples/watchedby/) is a complete local
demonstration with two small resource types.

## Known resource type

The example's Alert reconciler uses a typed messenger client:

```go
notifier := structured.NewNotifier(messengerClient, messengerShardID)

selector := selectors.NewLabelSelector[*api.Messenger](
    []resource.LabelSelector{{
        Key:      api.AlertLabel,
        Operator: resource.LabelSelectorOperatorIn,
        Values:   []string{alert.Name},
    }},
    alert.Namespace,
    selectors.WithFiltersSelectorOpts[*api.Messenger](
        func(m *api.Messenger) bool { return !m.Spec.Paused },
    ),
)

err := notifier.NotifyWatchedByResources(ctx, selector)
```

Here `messengerClient` is a `*controlloop.RemoteClient[*api.Messenger]`, and `api`
is the example's small [resource package](../../examples/watchedby/api/resources.go).
For one named target, use `selectors.NewNameSelector[*api.Messenger](name, namespace)`.

## Unknown resource type

When a target service owns the resource schema, use
`common/watchedby/unstructured` with a `common/unstructured_resource.UnstructuredClient`.
The client's metadata supplies the target Group, Kind and ShardID, while spec
and status remain raw JSON. The same selectors apply:

```go
notifier := unstructuredwatchedby.NewNotifier(client)
selector := selectors.NewNameSelector[*unstructured.UnstructuredResource]("chat", "demo")
err := notifier.NotifyWatchedByResources(ctx, selector)
```

If constructing a `RemoteClient[*unstructured.UnstructuredResource]`, set global
metadata **before** constructing the remote client: its constructor reads the
Group/Kind immediately. Serialize the entire set-and-construct sequence with a
shared mutex when initializing clients for different resource kinds.

## Contract

- Name, namespace and labels are selected by storage; extra predicates run locally.
- The shard belongs to the **target** resources and can differ from the source shard.
- `ListAll` reads every page before notification starts.
- Additional predicates are combined with AND.
- No matches is successful; `NotFound` during an update is skipped.
- Other errors stop the batch. Earlier updates are not rolled back.
- A retry can notify a dependent more than once.
- Success means the updates succeeded; dependent reconciles may still be running.

With state-manager, `Update` emits an event even if the spec is unchanged.
`UpdateStatus` does not emit that event. Avoid unconditional notification cycles
between resources; choose dependency edges and change guards deliberately.
