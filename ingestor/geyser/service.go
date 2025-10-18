package geyser

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
	"github.com/prometheus/client_golang/prometheus/promhttp"

	"github.com/rexbrahh/lp-indexer/ingestor/common"
	natsx "github.com/rexbrahh/lp-indexer/sinks/nats"

	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

// ClientInterface captures the subset of the geyser client used by the service.
type ClientInterface interface {
	Connect() error
	Subscribe(startSlot uint64) (<-chan *pb.SubscribeUpdate, <-chan error)
	Close() error
	Name() string
}

// Service wires the geyser client, processor, and publisher together.
type Service struct {
	client        ClientInterface
	processor     *Processor
	metricsAddr   string
	metricsServer *http.Server
	metricsStopCh chan struct{}
}

// NewService constructs a service using the provided geyser client configuration
// and JetStream publisher configuration. When metricsAddr is non-empty the
// service exposes Prometheus metrics on that address.
func NewService(geyserCfg *Config, natsCfg natsx.Config, metricsAddr string) (*Service, error) {
	if geyserCfg == nil {
		return nil, errors.New("geyser config is required")
	}
	client, err := NewClient(geyserCfg)
	if err != nil {
		return nil, fmt.Errorf("init geyser client: %w", err)
	}

	processor, _, server, stopCh, err := setupPipeline(natsCfg, metricsAddr)
	if err != nil {
		return nil, err
	}

	return &Service{
		client:        client,
		processor:     processor,
		metricsAddr:   metricsAddr,
		metricsServer: server,
		metricsStopCh: stopCh,
	}, nil
}

// Run connects to geyser, processes updates, and blocks until the context is
// cancelled or an unrecoverable error occurs.
func (s *Service) Run(ctx context.Context, startSlot uint64) error {
	if ctx == nil {
		ctx = context.Background()
	}

	if err := s.client.Connect(); err != nil {
		return fmt.Errorf("connect geyser: %w", err)
	}
	defer s.client.Close()

	updates, errs := s.client.Subscribe(startSlot)

	if s.metricsServer != nil {
		go func() {
			if err := s.metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
				// Metrics server errors are logged via stderr but do not kill the run loop.
				fmt.Printf("metrics server error: %v\n", err)
			}
			close(s.metricsStopCh)
		}()
	}

	for {
		select {
		case <-ctx.Done():
			s.shutdownMetrics()
			return ctx.Err()
		case err := <-errs:
			if err != nil {
				s.shutdownMetrics()
				return err
			}
		case update, ok := <-updates:
			if !ok {
				s.shutdownMetrics()
				return nil
			}
			if err := s.processor.HandleUpdate(ctx, update); err != nil {
				return err
			}
		}
	}
}

func (s *Service) shutdownMetrics() {
	if s.metricsServer == nil {
		return
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	_ = s.metricsServer.Shutdown(ctx)
	<-s.metricsStopCh
}

func buildMetricsServer(addr string, gatherer prometheus.Gatherer) *http.Server {
	if addr == "" {
		return nil
	}
	return &http.Server{
		Addr:              addr,
		Handler:           promhttp.HandlerFor(gatherer, promhttp.HandlerOpts{}),
		ReadHeaderTimeout: 5 * time.Second,
	}
}

func setupPipeline(natsCfg natsx.Config, metricsAddr string) (*Processor, prometheus.Registerer, *http.Server, chan struct{}, error) {
	publisher, err := natsx.NewPublisher(natsCfg)
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("init nats publisher: %w", err)
	}

	registry := prometheus.NewRegistry()
	registry.MustRegister(collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}))
	registry.MustRegister(collectors.NewGoCollector())

	slotCache := common.NewMemorySlotTimeCache()
	processor := NewProcessor(publisher, slotCache, registry)

	server := buildMetricsServer(metricsAddr, registry)
	stopCh := make(chan struct{})
	return processor, registry, server, stopCh, nil
}
