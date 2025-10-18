package geyser

import (
	"context"
	"errors"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"

	natsx "github.com/rexbrahh/lp-indexer/sinks/nats"
)

// FailoverService coordinates a primary/fallback client pair and feeds updates
// through a shared processor. When the primary stream exits with an error, the
// service attempts the fallback and periodically retries the primary.
type FailoverService struct {
	primary   ClientInterface
	fallback  ClientInterface
	processor *Processor
	metrics   *failoverMetrics

	metricsServer *http.Server
	metricsStopCh chan struct{}

	primaryRetryDelay  time.Duration
	fallbackRetryDelay time.Duration
}

// NewFailoverService constructs a failover service. When fallback is nil the
// service behaves identically to the single-client Service.
func NewFailoverService(primary ClientInterface, fallback ClientInterface, natsCfg natsx.Config, metricsAddr string) (*FailoverService, error) {
	if primary == nil {
		return nil, errors.New("primary client is required")
	}

	processor, registry, server, stopCh, err := setupPipeline(natsCfg, metricsAddr)
	if err != nil {
		return nil, err
	}

	metrics := newFailoverMetrics(registry)

	return &FailoverService{
		primary:            primary,
		fallback:           fallback,
		processor:          processor,
		metrics:            metrics,
		metricsServer:      server,
		metricsStopCh:      stopCh,
		primaryRetryDelay:  5 * time.Second,
		fallbackRetryDelay: 3 * time.Second,
	}, nil
}

// Run executes the failover loop until the context is cancelled. startSlot is
// forwarded to both clients (each is responsible for replaying recent slots).
func (s *FailoverService) Run(ctx context.Context, startSlot uint64) error {
	if ctx == nil {
		ctx = context.Background()
	}

	clients := []ClientInterface{s.primary}
	if s.fallback != nil {
		clients = append(clients, s.fallback)
	}

	if s.metricsServer != nil {
		go func() {
			if err := s.metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				fmt.Printf("metrics server error: %v\n", err)
			}
			close(s.metricsStopCh)
		}()
	}

	current := 0
	for {
		client := clients[current]
		s.metrics.setActive(client.Name())

		start := time.Now()
		err := s.runClient(ctx, client, startSlot)
		if errors.Is(err, context.Canceled) {
			s.shutdownMetrics()
			return ctx.Err()
		}
		if err != nil {
			s.metrics.recordFailure(client.Name())
			log.Printf("%s stream ended after %s: %v", client.Name(), time.Since(start).Round(time.Millisecond), err)
		}

		if len(clients) == 1 {
			time.Sleep(s.primaryRetryDelay)
			continue
		}

		// Toggle between primary and fallback.
		current = (current + 1) % len(clients)
		if current == 0 {
			time.Sleep(s.primaryRetryDelay)
		} else {
			time.Sleep(s.fallbackRetryDelay)
		}
	}
}

func (s *FailoverService) runClient(ctx context.Context, client ClientInterface, startSlot uint64) error {
	if err := client.Connect(); err != nil {
		return fmt.Errorf("connect %s: %w", client.Name(), err)
	}
	defer client.Close()

	updates, errs := client.Subscribe(startSlot)

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		case err := <-errs:
			if err != nil {
				return err
			}
		case update, ok := <-updates:
			if !ok {
				return errors.New("update stream closed")
			}
			if err := s.processor.HandleUpdate(ctx, update); err != nil {
				return err
			}
		}
	}
}

func (s *FailoverService) shutdownMetrics() {
	if s.metricsServer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.metricsServer.Shutdown(ctx)
	<-s.metricsStopCh
}

type failoverMetrics struct {
	activeSource prometheus.Gauge
	failures     *prometheus.CounterVec
}

func newFailoverMetrics(reg prometheus.Registerer) *failoverMetrics {
	if reg == nil {
		reg = prometheus.NewRegistry()
	}
	return &failoverMetrics{
		activeSource: promauto.With(reg).NewGauge(prometheus.GaugeOpts{
			Namespace: "dex",
			Subsystem: "ingestor",
			Name:      "active_source",
			Help:      "Indicates which ingest source is currently active (1=primary, 2=fallback)",
		}),
		failures: promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
			Namespace: "dex",
			Subsystem: "ingestor",
			Name:      "source_failures_total",
			Help:      "Count of stream failures per ingest source.",
		}, []string{"source"}),
	}
}

func (m *failoverMetrics) setActive(source string) {
	if m == nil {
		return
	}
	if source == "" {
		m.activeSource.Set(0)
		return
	}
	if source == "geyser" {
		m.activeSource.Set(1)
	} else {
		m.activeSource.Set(2)
	}
}

func (m *failoverMetrics) recordFailure(source string) {
	if m == nil {
		return
	}
	m.failures.WithLabelValues(source).Inc()
}
