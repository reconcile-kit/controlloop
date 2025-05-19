package controlloop

import (
	"context"
	resource "github.com/reconcile-kit/api"
	"time"
)

type Result struct {
	RequeueAfter time.Duration
	Requeue      bool
}

type Reconcile[T resource.Object[T]] interface {
	Reconcile(context.Context, T) (Result, error)
}
