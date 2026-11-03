// Package structured notifies dependent resources whose Go type is known.
//
// A Notifier lists resources in its target shard, applies a selector and calls
// RemoteClient.Update for every match. The backend must turn those updates into
// reconcile events, even when the resource spec is unchanged. UpdateStatus does
// not serve this purpose with state-manager.
//
// A notification does not wait for a dependent reconcile to finish. A missing
// resource is skipped; any other error stops the batch. Retrying may notify
// resources that were already updated, so dependent reconcilers must tolerate
// repeated notifications.
package structured
