// Package selectors selects resources for watchedby notifications.
//
// GetListOpts describes the name, namespace and label selection performed by
// the storage backend. FilterResources applies additional local predicates;
// it does not repeat the backend's selection. All local predicates must match.
//
// A notifier adds the target shard to the list options and collects every page
// before applying local filters. WithFilters mutates the selector, so create
// a new selector for each notification rather than sharing one across workers.
package selectors
