package controlloop

import (
	"context"
	"encoding/json"
	"fmt"
	prom "github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/reconcile-kit/api/resource"
	"github.com/reconcile-kit/controlloop/testdata/controller/metrics"
	cmetrics "github.com/reconcile-kit/controlloop/testdata/controller/metrics"
	wmetrics "github.com/reconcile-kit/controlloop/testdata/workqueue/metrics"
	"github.com/stretchr/testify/assert"
	"go.opentelemetry.io/otel"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"
	"io"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"net"
	"net/http"
)

/* -------------------------------------------------------------------------- */
/*                                  TEST Resource                             */
/* -------------------------------------------------------------------------- */
type testResource struct {
	resource.Resource
}

func (c *testResource) GetGK() resource.GroupKind {
	return resource.GroupKind{Group: "test", Kind: "test"}
}
func (c *testResource) DeepCopy() *testResource {
	return resource.DeepCopyStruct(c).(*testResource)
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

/* -------------------------------------------------------------------------- */
/*                                  TEST Box                                  */
/* -------------------------------------------------------------------------- */

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

/* -------------------------------------------------------------------------- */
/*                                  TEST Reconciler                           */
/* -------------------------------------------------------------------------- */

type fakeReconciler[T resource.Object[T]] struct {
	tb        *testBox
	callTest3 atomic.Int32
	unlock    chan struct{}
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

	if obj.GetName().Name == "test_metrics1" && obj.GetKillTimestamp() == "" {
		<-f.unlock
		fmt.Println("Requeue: test_metrics1 unlock")
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

/* -------------------------------------------------------------------------- */
/*                                  TEST Informer                             */
/* -------------------------------------------------------------------------- */

type TestInformer struct {
	ch      chan incomeMessage
	shardID string
}

func (i *TestInformer) Listen(f func(ctx context.Context, kind resource.GroupKind, objectKey resource.ObjectKey, messageType string, ack func())) {
	ctx := context.Background()
	for message := range i.ch {

		f(ctx, message.kind, message.name, message.messageType, func() {})
		time.Sleep(50 * time.Millisecond)
	}
}

func (i *TestInformer) ClearQueue(ctx context.Context) error {
	return nil
}

/* -------------------------------------------------------------------------- */
/*                                  TEST External Storage                     */
/* -------------------------------------------------------------------------- */

type testExternalStorage[T resource.Object[T]] struct {
}

func (t testExternalStorage[T]) Create(ctx context.Context, item T) error {
	return nil
}

func (t testExternalStorage[T]) Get(ctx context.Context, groupKind resource.GroupKind, objectKey resource.ObjectKey) (T, bool, error) {
	return newTestObject[T](groupKind, objectKey), true, nil
}

func (t testExternalStorage[T]) List(ctx context.Context, groupKind resource.GroupKind, listOpts resource.ListOpts) ([]T, error) {
	return []T{newTestObject[T](resource.GroupKind{Group: "test", Kind: "test"},
		resource.ObjectKey{Namespace: "test", Name: "test"})}, nil
}

func (t testExternalStorage[T]) ListPending(ctx context.Context, shardID string, groupKind resource.GroupKind) ([]T, error) {
	return []T{newTestObject[T](resource.GroupKind{Group: "test", Kind: "test"},
		resource.ObjectKey{Namespace: "test", Name: "testPending"}), newTestObject[T](resource.GroupKind{Group: "test", Kind: "test"},
		resource.ObjectKey{Namespace: "test", Name: "testPending2"})}, nil
}

func (t testExternalStorage[T]) Update(ctx context.Context, item T) error {
	return nil
}

func (t testExternalStorage[T]) UpdateStatus(ctx context.Context, item T) error {
	return nil
}

func (t testExternalStorage[T]) Delete(ctx context.Context, groupKind resource.GroupKind, objectKey resource.ObjectKey) error {
	return nil
}

func GetFreeTCPListener() (net.Listener, int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, err
	}
	addr := ln.Addr().(*net.TCPAddr)
	return ln, addr.Port, nil
}

/* -------------------------------------------------------------------------- */
/*                                   TESTS                                    */
/* -------------------------------------------------------------------------- */

func TestControlLoop_ReconcileAndStop(t *testing.T) {

	rec := &fakeReconciler[*testResource]{tb: newTestBox()}
	ctx := context.Background()

	externalStorage := &testExternalStorage[*testResource]{}

	sc, err := NewStorageController[*testResource]("test", externalStorage, NewMemoryStorage[*testResource]())
	if err != nil {
		t.Error(err)
	}

	informer := &TestInformer{ch: make(chan incomeMessage, 100), shardID: "test"}
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

func TestControlLoop_Metrics(t *testing.T) {
	ctx := context.Background()

	expectedLines := []string{
		`workqueue_depth{name="testResource",otel_scope_name="workqueue",otel_scope_schema_url="",otel_scope_version=""} 3`,
		`workqueue_adds_total{name="testResource",otel_scope_name="workqueue",otel_scope_schema_url="",otel_scope_version=""} 6`,
		`controlloop_reconcile_total{controller="testResource",otel_scope_name="controlloop",otel_scope_schema_url="",otel_scope_version="",result="success"} 2`,
		`controlloop_max_concurrent_reconciles{controller="testResource",otel_scope_name="controlloop",otel_scope_schema_url="",otel_scope_version=""} 1`,
		`controlloop_active_workers{controller="testResource",otel_scope_name="controlloop",otel_scope_schema_url="",otel_scope_version=""} 1`,
	}

	ln, _, err := GetFreeTCPListener()
	if err != nil {
		t.Error(err)
	}
	ln.Close()

	s, err := NewServer(Options{
		BindAddress: DefaultBindAddress + ":" + "8085",
	})
	if err != nil {
		t.Error(err)
	}

	errCh := make(chan error, 1)

	go func() {
		if err := s.Start(ctx); err != nil {
			errCh <- err
		}
	}()

	select {
	case err := <-errCh:
		if err != nil {
			t.Fatalf("server failed to start: %v", err)
		}
	case <-time.After(5 * time.Second):
	}

	unlock := make(chan struct{})
	rec := &fakeReconciler[*testResource]{tb: newTestBox(), unlock: unlock}

	externalStorage := &testExternalStorage[*testResource]{}

	sc, err := NewStorageController[*testResource]("test", externalStorage, NewMemoryStorage[*testResource](WithMetricsWorkqueueProvider(wmetrics.NewWorkqueueMetricsProvider(otel.Meter("workqueue")))))
	if err != nil {
		t.Error(err)
	}

	informer := &TestInformer{ch: make(chan incomeMessage, 100), shardID: "test"}
	for i := 1; i < 5; i++ {
		im := incomeMessage{
			kind:        resource.GroupKind{Group: "test", Kind: "test"},
			name:        resource.ObjectKey{Namespace: "test", Name: "test_metrics" + fmt.Sprint(i)},
			messageType: resource.MessageTypeUpdate,
		}
		informer.ch <- im
	}
	err = NewStorageInformer("test", informer, []Receiver{sc}).Run(ctx)
	if err != nil {
		t.Error(err)
	}

	cl := New[*testResource](rec, sc, WithMetricsProvider(cmetrics.NewControllerMetrics(otel.Meter("controlloop"))))

	cl.Run()

	registry := NewStorageSet()
	SetStorage[*testResource](registry, sc)

	assert.Eventually(t, func() bool {
		url := "http://" + DefaultBindAddress + ":" + "8085" + defaultMetricsEndpoint
		resp, err := http.Get(url)
		if err != nil {
			t.Logf("failed to GET %s: %v", url, err)
			return false
		}
		defer resp.Body.Close()
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			t.Logf("failed to read body: %v", err)
			return false
		}

		text := string(body)

		for _, line := range expectedLines {
			if !strings.Contains(text, line) {
				t.Logf("metric line not found: %s", line)
				return false
			}
		}

		return true

	}, time.Second*50, 10*time.Millisecond, "reconcile must be called once")

	t.Logf("Stopping")
	close(unlock)
	cl.Stop()

	assert.Equal(t, 0, cl.Queue.len())
}

/* -------------------------------------------------------------------------- */
/*                                  TEST Metrics Server                       */
/* -------------------------------------------------------------------------- */

const defaultMetricsEndpoint = "/observability"

// DefaultBindAddress is the default bind address for the testdata server.
var DefaultBindAddress = "localhost"

type Server interface {
	Start(ctx context.Context) error
}

type Options struct {
	// BindAddress is the bind address for the testdata server.
	// It will be defaulted to ":8080" if unspecified.
	// Set this to "0" to disable the testdata server.
	BindAddress string
	// ListenConfig contains options for listening to an address on the metric server.
	ListenConfig net.ListenConfig

	EnableGoRuntime bool // GC, goroutines, mem и т.п.
	EnableHostProc  bool // CPU, mem, load, процессные метрики

	EnableOTLPPush bool
	OTLPEndpoint   string // "collector:4317"
	OTLPInsecure   bool   // true если без TLS
	OTLPInterval   time.Duration
}

func (o *Options) setDefaults() {
	if o.BindAddress == "" {
		o.BindAddress = DefaultBindAddress
	}
	if o.EnableGoRuntime == false && o.EnableHostProc == false {
		o.EnableGoRuntime = true
		o.EnableHostProc = true
	}
	if o.EnableOTLPPush && o.OTLPInterval <= 0 {
		o.OTLPInterval = 5 * time.Second
	}
}

func NewServer(o Options) (Server, error) {
	o.setDefaults()
	readers := []sdkmetric.Reader{}

	// Pull: Prometheus /testdata
	reg := prom.NewRegistry()
	promExp, err := otelprom.New(otelprom.WithRegisterer(reg))
	if err != nil {
		return nil, fmt.Errorf("prometheus exporter init: %w", err)
	}
	readers = append(readers, promExp)

	mpOpts := make([]sdkmetric.Option, 0, len(readers))
	for _, r := range readers {
		mpOpts = append(mpOpts, sdkmetric.WithReader(r))
	}
	mpOpts = append(mpOpts, metrics.ViewOptions()...)

	mp := sdkmetric.NewMeterProvider(mpOpts...)
	otel.SetMeterProvider(mp)
	if o.BindAddress == "0" {
		return nil, nil
	}

	mux := http.NewServeMux()
	mux.Handle(defaultMetricsEndpoint, promhttp.HandlerFor(reg, promhttp.HandlerOpts{}))
	srv := newServer(mux)

	return &defaultServer{options: o, srv: srv}, nil
}

type defaultServer struct {
	options Options
	mu      sync.RWMutex
	srv     *http.Server
}

func (s *defaultServer) Start(ctx context.Context) error {
	ln, err := s.createListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to start testdata server: failed to create listener: %w", err)
	}
	idleConnsClosed := make(chan struct{})
	go func() {
		<-ctx.Done()
		ctxShutdown, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_ = s.srv.Shutdown(ctxShutdown)

		close(idleConnsClosed)
	}()

	if err := s.srv.Serve(ln); err != nil && err != http.ErrServerClosed {
		return err
	}

	<-idleConnsClosed
	return nil
}

func (s *defaultServer) createListener(ctx context.Context) (net.Listener, error) {
	return s.options.ListenConfig.Listen(ctx, "tcp", s.options.BindAddress)
}

// New returns a new server with sane defaults.
func newServer(handler http.Handler) *http.Server {
	return &http.Server{
		Handler:           handler,
		MaxHeaderBytes:    1 << 20,
		IdleTimeout:       90 * time.Second, // matches http.DefaultTransport keep-alive timeout
		ReadHeaderTimeout: 32 * time.Second,
	}
}
