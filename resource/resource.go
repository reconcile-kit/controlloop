package resource

import (
	"errors"
	"sync/atomic"
	"time"
)

type Object[T any] interface {
	GetGeneration() int64
	IncGeneration()
	SetKillTimestamp(time time.Time)
	GetKillTimestamp() string
	SetDeletionTimestamp(time.Time)
	GetDeletionTimestamp() string
	GetName() ObjectKey
	GetVersion() int
	GetCurrentVersion() int
	SetCurrentVersion(int)
	SetResourceGroup(resourceGroup string)
	SetKind(kind string)

	DeepCopy() T
}

const TimeFormat = "2006-01-02 15:04:05"

var ConflictError = errors.New("versions conflict")
var NotFoundError = errors.New("resource not found")

type GroupKind struct {
	Group string
	Kind  string
}

type ObjectKey struct {
	Namespace string
	Name      string
}

type ListOpts struct {
	Group     string `json:"resource_group"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace"`
	Name      string `json:"name"`
	ShardID   string `json:"shard_id"`
}

type Resource struct {
	ResourceGroup     string `json:"resource_group"`
	Kind              string `json:"kind"`
	Namespace         string `json:"namespace"`
	Name              string `json:"name"`
	KillTimestamp     string
	DeletionTimestamp string `json:"deletion_timestamp"`
	Generation        int64
	Finalizers        []string          `json:"finalizers"`
	Annotations       map[string]string `json:"annotations"`
	Labels            map[string]string `json:"labels"`
	Version           int               `json:"version"`
	CurrentVersion    int               `json:"current_version"`
}

func NewResource(name ObjectKey) *Resource {
	return &Resource{Name: name.Name, Namespace: name.Namespace}
}

func (r *Resource) SetKillTimestamp(time time.Time) {
	if r.KillTimestamp == "" {
		r.KillTimestamp = time.Format(TimeFormat)
	}
}

func (r *Resource) GetKillTimestamp() string {
	return r.KillTimestamp
}

func (r *Resource) SetDeletionTimestamp(time time.Time) {
	if r.DeletionTimestamp == "" {
		r.DeletionTimestamp = time.Format(TimeFormat)
	}
}

func (r *Resource) GetDeletionTimestamp() string {
	return r.DeletionTimestamp
}

func (r *Resource) IncGeneration() {
	atomic.AddInt64(&r.Generation, 1)
}

func (r *Resource) GetGeneration() int64 {
	return atomic.LoadInt64(&r.Generation)
}

func (r *Resource) GetName() ObjectKey {
	return ObjectKey{
		Namespace: r.Namespace,
		Name:      r.Name,
	}
}

func (r *Resource) GetVersion() int {
	return r.Version
}

func (r *Resource) GetCurrentVersion() int {
	return r.CurrentVersion
}

func (r *Resource) SetCurrentVersion(v int) {
	r.CurrentVersion = v
}

func (r *Resource) SetResourceGroup(resourceGroup string) {
	r.ResourceGroup = resourceGroup
}

func (r *Resource) SetKind(kind string) {
	r.Kind = kind
}
