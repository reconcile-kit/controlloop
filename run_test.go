// controlloop_test.go
package controlloop

import (
	"context"
	"github.com/reconcile-kit/controlloop/resource"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

type testResource struct {
	resource.Resource
}

func (c *testResource) DeepCopy() *testResource {
	return DeepCopyStruct(c).(*testResource)
}

type fakeReconciler[T resource.Object[T]] struct {
	callCnt atomic.Int32
}

func (f *fakeReconciler[T]) Reconcile(ctx context.Context, obj T) (Result, error) {
	f.callCnt.Add(1)
	return Result{}, nil
}

/* -------------------------------------------------------------------------- */
/*                                   TESTS                                    */
/* -------------------------------------------------------------------------- */

func TestControlLoop_ReconcileAndStop(t *testing.T) {
	mem := NewMemoryStorage[*testResource]() // маленький in-mem стор (ваш тип)
	rec := &fakeReconciler[*testResource]{}
	obj := &testResource{}
	obj.Name = "test"
	mem.Add(obj)

	cl := New[*testResource](rec, mem /* no options */)

	go cl.Run()

	assert.Eventually(t, func() bool {
		return rec.callCnt.Load() == 1
	}, time.Second, 10*time.Millisecond, "reconcile must be called once")

	cl.Stop()

	assert.Equal(t, 0, cl.Queue.len())
}
