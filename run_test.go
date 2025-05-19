package controlloop

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/reconcile-kit/api/resource"
	"sync"
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
	tb        *testBox
	callTest3 atomic.Int32
}

type testBox struct {
	m         *sync.Mutex
	resources map[string]bool
}

func (tb *testBox) Add(name string) {
	defer tb.m.Unlock()
	tb.m.Lock()
	tb.resources[name] = true
}
func (tb *testBox) Exist(name string) bool {
	defer tb.m.Unlock()
	tb.m.Lock()
	_, ok := tb.resources[name]
	return ok
}

func newTestBox() *testBox {
	return &testBox{
		m:         &sync.Mutex{},
		resources: map[string]bool{},
	}
}

func (f *fakeReconciler[T]) Reconcile(ctx context.Context, obj *testResource) (Result, error) {
	if obj.GetName().Name == "test3" && f.callTest3.Load() != 4 {
		f.callTest3.Add(1)
		fmt.Println("Requeue: test3 requeue")
		return Result{RequeueAfter: time.Millisecond * 30}, nil
	}

	if obj.GetName().Name == "test4" && obj.GetKillTimestamp() != "" {
		return Result{}, nil
	}

	if obj.GetName().Name == "test4" {
		fmt.Println("Requeue: test4 requeue")
		return Result{RequeueAfter: time.Second * 2}, nil
	}

	f.tb.Add(obj.Name)
	fmt.Println("Current: ", obj.Name)

	return Result{}, nil
}

type incomeMessage struct {
	kind        resource.GroupKind
	name        resource.ObjectKey
	messageType string
}

type TestInformer struct {
	ch chan incomeMessage
}

func (i *TestInformer) Listen(shardID string, f func(ctx context.Context, kind resource.GroupKind, objectKey resource.ObjectKey, messageType string, ack func())) {
	ctx := context.Background()
	for message := range i.ch {

		f(ctx, message.kind, message.name, message.messageType, func() {})
		time.Sleep(50 * time.Millisecond)
	}
}

func (i *TestInformer) ClearQueue(ctx context.Context) error {
	return nil
}

type testExternalStorage[T resource.Object[T]] struct {
}

func (t testExternalStorage[T]) Create(ctx context.Context, item T) (T, error) {
	return item, nil
}

func newTestObject[T resource.Object[T]](groupKind resource.GroupKind, objectKey resource.ObjectKey) T {
	body := []byte(fmt.Sprintf(`{"name":"%s", "resource_group":"%s", "kind":"%s", "namespace":"%s"}`,
		objectKey.Name,
		groupKind.Group,
		groupKind.Kind,
		objectKey.Namespace,
	))
	var zero T
	err := json.Unmarshal(body, &zero)
	if err != nil {
		panic(err)
	}
	return zero
}

func (t testExternalStorage[T]) Get(ctx context.Context, shardID string, groupKind resource.GroupKind, objectKey resource.ObjectKey) (T, bool, error) {
	return newTestObject[T](groupKind, objectKey), true, nil
}

func (t testExternalStorage[T]) List(ctx context.Context, listOpts resource.ListOpts) ([]T, error) {
	return []T{newTestObject[T](resource.GroupKind{Group: "test", Kind: "test"},
		resource.ObjectKey{Namespace: "test", Name: "test"})}, nil
}

func (t testExternalStorage[T]) ListPending(ctx context.Context, shardID string, groupKind resource.GroupKind) ([]T, error) {
	return []T{newTestObject[T](resource.GroupKind{Group: "test", Kind: "test"},
		resource.ObjectKey{Namespace: "test", Name: "testPending"}), newTestObject[T](resource.GroupKind{Group: "test", Kind: "test"},
		resource.ObjectKey{Namespace: "test", Name: "testPending2"})}, nil
}

func (t testExternalStorage[T]) Update(ctx context.Context, item T) (T, error) {
	return item, nil
}

func (t testExternalStorage[T]) UpdateStatus(ctx context.Context, item T) (T, error) {
	return item, nil
}

func (t testExternalStorage[T]) Delete(ctx context.Context, shardID string, groupKind resource.GroupKind, objectKey resource.ObjectKey) error {
	return nil
}

/* -------------------------------------------------------------------------- */
/*                                   TESTS                                    */
/* -------------------------------------------------------------------------- */

func TestControlLoop_ReconcileAndStop(t *testing.T) {
	rec := &fakeReconciler[*testResource]{tb: newTestBox()}
	obj := &testResource{}
	obj.Name = "test"
	obj.Namespace = "test"
	obj.Kind = "test"
	obj.ResourceGroup = "test"
	ctx := context.Background()

	externalStorage := &testExternalStorage[*testResource]{}

	sc, err := NewStorageController[*testResource]("test", resource.GroupKind{
		Group: "test",
		Kind:  "test",
	}, externalStorage, NewMemoryStorage[*testResource]())
	if err != nil {
		t.Error(err)
	}

	informer := &TestInformer{ch: make(chan incomeMessage, 100)}
	for i := 1; i < 5; i++ {
		im := incomeMessage{
			kind:        resource.GroupKind{Group: "test", Kind: "test"},
			name:        resource.ObjectKey{Namespace: "test", Name: "test" + fmt.Sprint(i)},
			messageType: resource.MessageTypeUpdate,
		}
		informer.ch <- im
	}
	err = NewStorageInformer("test", informer, []Receiver{sc}).Run(ctx)
	if err != nil {
		t.Error(err)
	}

	cl := New[*testResource](rec, sc /* no options */)

	cl.Run()

	im := incomeMessage{
		kind:        resource.GroupKind{Group: "test", Kind: "test"},
		name:        resource.ObjectKey{Namespace: "test", Name: "test5"},
		messageType: resource.MessageTypeUpdate,
	}
	informer.ch <- im

	//imDelete := incomeMessage{
	//	kind:        resource.GroupKind{Group: "test", Kind: "test"},
	//	name:        resource.ObjectKey{Namespace: "test", Name: "test4"},
	//	messageType: resource.MessageTypeDelete,
	//}
	//informer.ch <- imDelete

	registry := NewStorageSet()
	SetStorage[*testResource](registry, sc)
	testResourceClient, ok := GetStorage[*testResource](registry)
	if !ok {
		t.Error("storage set is empty")
	}

	err = testResourceClient.Create(ctx, &testResource{Resource: resource.Resource{
		ResourceGroup: "test",
		Kind:          "test",
		Namespace:     "test",
		Name:          "manualCreate",
	}})
	if !ok {
		t.Error("create resource  error", err)
	}

	assert.Eventually(t, func() bool {
		switch {
		case !rec.tb.Exist("manualCreate"):
			return false
		case !rec.tb.Exist("testPending"):
			return false
		case !rec.tb.Exist("testPending2"):
			return false
		case !rec.tb.Exist("test1"):
			return false
		case !rec.tb.Exist("test2"):
			return false
		case !rec.tb.Exist("test3"):
			return false
		case !rec.tb.Exist("test5"):
			return false
		}
		return true

	}, time.Second*5, 10*time.Millisecond, "reconcile must be called once")

	fmt.Println("Stopping")

	cl.Stop()

	assert.Equal(t, 0, cl.Queue.len())
}
