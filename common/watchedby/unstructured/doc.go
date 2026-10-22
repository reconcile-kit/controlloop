// Package unstructured notifies dependent resources without importing their
// concrete Go types. Resource spec and status remain raw JSON.
//
// Its Notifier uses the client's metadata to select the target shard, then
// updates each selected resource. The backend must deliver those updates to the
// target control loop. A missing resource is skipped; other errors stop the
// batch, and retries may notify already updated resources again.
package unstructured
