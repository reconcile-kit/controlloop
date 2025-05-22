package server

import (
	"context"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/reconcile-kit/controlloop/observability/registry"
	"net"
	"net/http"
	"sync"
	"time"
)

const (
	defaultMetricsEndpoint = "/observability"
)

// DefaultBindAddress is the default bind address for the observability server.
var DefaultBindAddress = ":8080"

// Server is a server that serves observability.
type Server interface {
	// Start runs the server.
	// It will install the observability related resources depending on the server configuration.
	Start(ctx context.Context) error
}

type Options struct {
	// BindAddress is the bind address for the observability server.
	// It will be defaulted to ":8080" if unspecified.
	// Set this to "0" to disable the observability server.
	BindAddress string

	// ListenConfig contains options for listening to an address on the metric server.
	ListenConfig net.ListenConfig
}

// setDefaults does defaulting for the Server.
func (o *Options) setDefaults() {
	if o.BindAddress == "" {
		o.BindAddress = DefaultBindAddress
	}
}

// NewServer constructs a new observability.Server from the provided options.
func NewServer(o Options) (Server, error) {
	o.setDefaults()

	// Skip server creation if observability are disabled.
	if o.BindAddress == "0" {
		return nil, nil
	}

	return &defaultServer{
		options: o,
	}, nil
}

// defaultServer is the default implementation used for Server.
type defaultServer struct {
	options Options

	// mu protects access to the bindAddr field.
	mu sync.RWMutex
}

// Start runs the server.
// It will install the observability related resources depend on the server configuration.
func (s *defaultServer) Start(ctx context.Context) error {
	listener, err := s.createListener(ctx)
	if err != nil {
		return fmt.Errorf("failed to start observability server: failed to create listener: %w", err)
	}

	mux := http.NewServeMux()

	handler := promhttp.HandlerFor(registry.Registry, promhttp.HandlerOpts{
		ErrorHandling: promhttp.HTTPErrorOnError,
	})
	mux.Handle(defaultMetricsEndpoint, handler)

	srv := newServer(mux)

	idleConnsClosed := make(chan struct{})
	go func() {
		<-ctx.Done()

		ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
		defer cancel()
		if err := srv.Shutdown(ctx); err != nil {
			// TODO add logs
		}
		close(idleConnsClosed)
	}()

	if err := srv.Serve(listener); err != nil && err != http.ErrServerClosed {
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
