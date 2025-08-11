package server

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/reconcile-kit/controlloop/observability/controller/metrics"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	otelprom "go.opentelemetry.io/otel/exporters/prometheus"
	sdkmetric "go.opentelemetry.io/otel/sdk/metric"

	"go.opentelemetry.io/contrib/instrumentation/host"
	"go.opentelemetry.io/contrib/instrumentation/runtime"
)

const defaultMetricsEndpoint = "/observability"

// DefaultBindAddress is the default bind address for the observability server.
var DefaultBindAddress = ":8080"

type Server interface {
	Start(ctx context.Context) error
}

type Options struct {
	// BindAddress is the bind address for the observability server.
	// It will be defaulted to ":8080" if unspecified.
	// Set this to "0" to disable the observability server.
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
	if o.BindAddress == "0" {
		return nil, nil
	}
	return &defaultServer{options: o}, nil
}

type defaultServer struct {
	options Options
	mu      sync.RWMutex
}

func (s *defaultServer) Start(ctx context.Context) error {
	ln, err := s.createListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to start observability server: failed to create listener: %w", err)
	}

	readers := []sdkmetric.Reader{}

	// Pull: Prometheus /observability
	promExp, err := otelprom.New()
	if err != nil {
		return fmt.Errorf("prometheus exporter init: %w", err)
	}
	readers = append(readers, promExp)

	// Push: OTLP (опционально)
	if s.options.EnableOTLPPush && s.options.OTLPEndpoint != "" {
		opts := []otlpmetricgrpc.Option{otlpmetricgrpc.WithEndpoint(s.options.OTLPEndpoint)}
		if s.options.OTLPInsecure {
			opts = append(opts, otlpmetricgrpc.WithInsecure())
		}
		otlpExp, err := otlpmetricgrpc.New(ctx, opts...)
		if err != nil {
			return fmt.Errorf("otlp exporter init: %w", err)
		}
		readers = append(readers, sdkmetric.NewPeriodicReader(otlpExp, sdkmetric.WithInterval(s.options.OTLPInterval)))
	}

	mpOpts := make([]sdkmetric.Option, 0, len(readers))
	for _, r := range readers {
		mpOpts = append(mpOpts, sdkmetric.WithReader(r))
	}
	mpOpts = append(mpOpts, metrics.ViewOptions()...)

	mp := sdkmetric.NewMeterProvider(mpOpts...)
	otel.SetMeterProvider(mp)

	if s.options.EnableGoRuntime {
		// метрики Go-рантайма (GC, allocs, goroutines, heap/stack, паузы и т.п.)
		_ = runtime.Start(
			runtime.WithMinimumReadMemStatsInterval(10 * time.Second),
		)
	}
	if s.options.EnableHostProc {
		// метрики хоста/процесса: CPU, load, mem, диски, сеть, процессные
		var err error
		err = host.Start()
		if err != nil {
			return fmt.Errorf("host metrics init: %w", err)
		}
	}

	mux := http.NewServeMux()
	mux.Handle(defaultMetricsEndpoint, promExp)

	srv := newServer(mux)

	idleConnsClosed := make(chan struct{})
	go func() {
		<-ctx.Done()
		ctxShutdown, cancel := context.WithTimeout(context.Background(), time.Minute)
		defer cancel()
		_ = srv.Shutdown(ctxShutdown)
		_ = mp.Shutdown(ctxShutdown)

		close(idleConnsClosed)
	}()

	if err := srv.Serve(ln); err != nil && err != http.ErrServerClosed {
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
