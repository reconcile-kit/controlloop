package controlloop

import (
	"context"
	"errors"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/reconcile-kit/api/resource"
)

// failingStorage stands in for a store that cannot be reached — a network blip
// between a shard and the state manager, which is the ordinary way this fails.
type failingStorage[T resource.Object[T]] struct {
	testExternalStorage[T]
}

func (failingStorage[T]) Get(context.Context, resource.GroupKind, resource.ObjectKey) (T, bool, error) {
	var zero T
	return zero, false, errors.New("state manager unreachable")
}

// recordingLogger keeps what was reported so a test can read it.
type recordingLogger struct {
	mu    sync.Mutex
	lines []string
}

func (l *recordingLogger) Error(args ...interface{}) {
	l.mu.Lock()
	defer l.mu.Unlock()
	for _, a := range args {
		if s, ok := a.(string); ok {
			l.lines = append(l.lines, s)
		}
	}
}

func (l *recordingLogger) Info(...interface{}) {}

func (l *recordingLogger) joined() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Join(l.lines, "\n")
}

// An event that could not be taken must stay in the queue and must be
// reported. Acknowledging it would drop it for good; staying quiet about it
// leaves a shard that never learns the object exists, with nothing anywhere
// saying why — the only trace being a pending entry in the queue, which is not
// where anyone looks when an object fails to arrive.
func TestUnreceivedEventIsReportedAndNotAcknowledged(t *testing.T) {
	sc, err := NewStorageController[*testResource]("test", &failingStorage[*testResource]{}, NewMemoryStorage[*testResource]())
	if err != nil {
		t.Fatal(err)
	}

	log := &recordingLogger{}
	informer := NewStorageInformer("test", &TestInformer{ch: make(chan incomeMessage, 1), shardID: "test"},
		[]Receiver{sc}, WithInformerLogger(log))

	var acknowledged atomic.Bool
	informer.receiveMessages(context.Background(),
		resource.GroupKind{Group: "test", Kind: "test"},
		resource.ObjectKey{Namespace: "ns", Name: "obj"},
		resource.MessageTypeUpdate,
		func() { acknowledged.Store(true) })

	// receiveMessages answers on a goroutine of its own.
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && log.joined() == "" {
		time.Sleep(10 * time.Millisecond)
	}

	if acknowledged.Load() {
		t.Error("an event that was never taken was acknowledged, so the queue will not hand it out again")
	}

	reported := log.joined()
	if reported == "" {
		t.Fatal("nothing was reported: this is the silent loss the report exists to prevent")
	}
	// The report has to say which object, or it cannot be acted on.
	for _, want := range []string{"ns", "obj", "test", "state manager unreachable"} {
		if !strings.Contains(reported, want) {
			t.Errorf("report does not mention %q: %s", want, reported)
		}
	}
}

// The ordinary path must be untouched: a taken event is acknowledged and
// nothing is reported.
func TestReceivedEventIsAcknowledgedQuietly(t *testing.T) {
	sc, err := NewStorageController[*testResource]("test", &testExternalStorage[*testResource]{}, NewMemoryStorage[*testResource]())
	if err != nil {
		t.Fatal(err)
	}

	log := &recordingLogger{}
	informer := NewStorageInformer("test", &TestInformer{ch: make(chan incomeMessage, 1), shardID: "test"},
		[]Receiver{sc}, WithInformerLogger(log))

	var acknowledged atomic.Bool
	informer.receiveMessages(context.Background(),
		resource.GroupKind{Group: "test", Kind: "test"},
		resource.ObjectKey{Namespace: "ns", Name: "obj"},
		resource.MessageTypeUpdate,
		func() { acknowledged.Store(true) })

	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) && !acknowledged.Load() {
		time.Sleep(10 * time.Millisecond)
	}

	if !acknowledged.Load() {
		t.Error("a taken event was not acknowledged, so it will be handed out again")
	}
	if got := log.joined(); got != "" {
		t.Errorf("an ordinary delivery was reported: %s", got)
	}
}
