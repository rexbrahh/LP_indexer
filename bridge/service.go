package bridge

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	nats "github.com/nats-io/nats.go"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"golang.org/x/sync/errgroup"

	"github.com/rexbrahh/lp-indexer/observability"
)

// SubjectMapper maps canonical subjects to legacy equivalents. Returning ok=false
// drops the message (still acknowledging the source) which is useful while the
// legacy surface is narrowed during cutover.
type SubjectMapper func(subject string) (mapped string, ok bool)

// Option customises Service behaviour.
type Option func(*Service)

const (
	defaultFetchBatch     = 64
	defaultFetchWait      = 100 * time.Millisecond
	defaultPublishTimeout = 2 * time.Second
)

// Service wires source and target JetStream connections and forwards messages.
type Service struct {
	cfg            Config
	mapper         SubjectMapper
	fetchBatch     int
	fetchWait      time.Duration
	publishTimeout time.Duration
	customMapper   bool
	metrics        *serviceMetrics
	metricsServer  string
}

// New creates a Service with validated configuration.
func New(cfg Config, opts ...Option) (*Service, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	svc := &Service{
		cfg:            cfg,
		mapper:         defaultSubjectMapper,
		fetchBatch:     defaultFetchBatch,
		fetchWait:      defaultFetchWait,
		publishTimeout: defaultPublishTimeout,
		metricsServer:  cfg.MetricsAddr,
	}

	for _, opt := range opts {
		if opt != nil {
			opt(svc)
		}
	}

	if !svc.customMapper && len(cfg.SubjectMappings) > 0 {
		mapper, err := mapperFromMappings(cfg.SubjectMappings)
		if err != nil {
			return nil, err
		}
		svc.mapper = mapper
	}

	if svc.mapper == nil {
		svc.mapper = defaultSubjectMapper
	}
	if svc.metrics == nil {
		svc.metrics = newServiceMetrics(nil, nil)
	}
	if svc.fetchBatch <= 0 {
		return nil, fmt.Errorf("fetch batch must be positive")
	}
	if svc.fetchWait <= 0 {
		svc.fetchWait = defaultFetchWait
	}

	return svc, nil
}

// WithSubjectMapper overrides the default identity subject mapping.
func WithSubjectMapper(mapper SubjectMapper) Option {
	return func(s *Service) {
		if mapper != nil {
			s.mapper = mapper
			s.customMapper = true
		}
	}
}

// WithMetricsRegisterer allows callers to provide a Prometheus registerer.
// When omitted an isolated registry is used to avoid duplicate registrations
// in multi-tenant binaries or tests.
func WithMetricsRegisterer(reg prometheus.Registerer, gatherer prometheus.Gatherer) Option {
	return func(s *Service) {
		s.metrics = newServiceMetrics(reg, gatherer)
	}
}

// WithMetricsServer configures an HTTP endpoint (e.g. ":9090") that exposes
// Prometheus metrics registered via WithMetricsRegisterer or the default
// isolated registry.
func WithMetricsServer(addr string) Option {
	return func(s *Service) {
		s.metricsServer = addr
	}
}

// Run starts the bridge until the context is cancelled.
func (s *Service) Run(ctx context.Context) error {
	if ctx == nil {
		ctx = context.Background()
	}

	sourceConn, err := nats.Connect(s.cfg.SourceURL)
	if err != nil {
		return fmt.Errorf("connect source nats: %w", err)
	}
	defer sourceConn.Close()

	targetConn, err := nats.Connect(s.cfg.TargetURL)
	if err != nil {
		sourceConn.Close()
		return fmt.Errorf("connect target nats: %w", err)
	}
	defer targetConn.Close()

	sourceJS, err := sourceConn.JetStream()
	if err != nil {
		return fmt.Errorf("source jetstream context: %w", err)
	}
	targetJS, err := targetConn.JetStream()
	if err != nil {
		return fmt.Errorf("target jetstream context: %w", err)
	}

	info, err := sourceJS.StreamInfo(s.cfg.SourceStream)
	if err != nil {
		return fmt.Errorf("describe source stream %q: %w", s.cfg.SourceStream, err)
	}

	subjects := collectSubjects(&info.Config)
	if len(subjects) == 0 {
		return fmt.Errorf("source stream %q exposes no subjects", s.cfg.SourceStream)
	}

	g, runCtx := errgroup.WithContext(ctx)

	if s.metricsServer != "" && s.metrics != nil && s.metrics.gatherer != nil {
		metricsSrv := &http.Server{
			Addr:    s.metricsServer,
			Handler: promhttp.HandlerFor(s.metrics.gatherer, promhttp.HandlerOpts{}),
		}
		g.Go(func() error {
			err := metricsSrv.ListenAndServe()
			if err == nil || errors.Is(err, http.ErrServerClosed) {
				return nil
			}
			return fmt.Errorf("metrics server: %w", err)
		})
		g.Go(func() error {
			<-runCtx.Done()
			shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			_ = metricsSrv.Shutdown(shutdownCtx)
			return nil
		})
	}

	for _, subj := range subjects {
		subject := subj
		durable := durableName(subject)

		sub, err := sourceJS.PullSubscribe(subject, durable, nats.BindStream(s.cfg.SourceStream), nats.ManualAck())
		if err != nil {
			return fmt.Errorf("pull subscribe %q: %w", subject, err)
		}

		g.Go(func() error {
			defer sub.Unsubscribe()
			return s.consumeLoop(runCtx, sub, targetJS)
		})
	}

	if err := g.Wait(); err != nil {
		if errors.Is(err, context.Canceled) {
			return context.Canceled
		}
		return err
	}
	return nil
}

// Config returns the service configuration.
func (s *Service) Config() Config {
	return s.cfg
}

func (s *Service) consumeLoop(ctx context.Context, sub *nats.Subscription, targetJS nats.JetStreamContext) error {
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		msgs, err := sub.Fetch(s.fetchBatch, nats.MaxWait(s.fetchWait))
		if err != nil {
			if errors.Is(err, nats.ErrTimeout) {
				continue
			}
			if ctxErr := ctx.Err(); ctxErr != nil {
				return ctxErr
			}
			return err
		}

		for _, msg := range msgs {
			if err := s.forward(ctx, targetJS, msg); err != nil {
				return err
			}
		}
	}
}

func (s *Service) forward(ctx context.Context, targetJS nats.JetStreamContext, msg *nats.Msg) error {
	if s.metrics != nil {
		if md, err := msg.Metadata(); err == nil {
			lag := time.Since(md.Timestamp).Seconds()
			if lag < 0 {
				lag = 0
			}
			s.metrics.observeLag(msg.Subject, lag)
		}
	}

	mapped, ok := s.mapper(msg.Subject)
	if !ok {
		if s.metrics != nil {
			s.metrics.incDropped(msg.Subject)
		}
		return msg.Ack()
	}

	publish := &nats.Msg{Subject: mapped, Data: msg.Data}
	if len(msg.Header) > 0 {
		publish.Header = cloneHeader(msg.Header)
	}

	pubCtx, cancel := s.publishContext(ctx)
	ack, err := targetJS.PublishMsg(publish, nats.Context(pubCtx), nats.ExpectStream(s.cfg.TargetStream))
	cancel()
	if err != nil {
		if s.metrics != nil {
			s.metrics.incPublishError(mapped)
		}
		if errors.Is(err, context.Canceled) {
			return context.Canceled
		}
		return fmt.Errorf("publish to %s: %w", mapped, err)
	}
	if ack != nil && ack.Stream != "" && ack.Stream != s.cfg.TargetStream {
		return fmt.Errorf("publish to %s: unexpected ack stream %q", mapped, ack.Stream)
	}

	if err := msg.Ack(); err != nil {
		return fmt.Errorf("ack source message: %w", err)
	}
	if s.metrics != nil {
		s.metrics.incForwarded(mapped)
	}
	return nil
}

func (s *Service) publishContext(parent context.Context) (context.Context, context.CancelFunc) {
	if s.publishTimeout <= 0 {
		return parent, func() {}
	}
	return context.WithTimeout(parent, s.publishTimeout)
}

func collectSubjects(cfg *nats.StreamConfig) []string {
	if cfg == nil {
		return nil
	}
	if len(cfg.Subjects) > 0 {
		return append([]string(nil), cfg.Subjects...)
	}
	return nil
}

func defaultSubjectMapper(subject string) (string, bool) {
	return subject, true
}

func durableName(subject string) string {
	cleaned := strings.Map(func(r rune) rune {
		switch r {
		case '.', '*', '>', ' ', ':':
			return '_'
		default:
			return r
		}
	}, subject)
	cleaned = strings.Trim(cleaned, "_")
	if cleaned == "" {
		cleaned = "all"
	}
	if len(cleaned) > 64 {
		cleaned = cleaned[:64]
	}
	return "bridge_" + cleaned
}

func cloneHeader(h nats.Header) nats.Header {
	if len(h) == 0 {
		return nil
	}
	dup := nats.Header{}
	for k, values := range h {
		copied := make([]string, len(values))
		copy(copied, values)
		dup[k] = copied
	}
	return dup
}

type serviceMetrics struct {
	forwarded  *prometheus.CounterVec
	dropped    *prometheus.CounterVec
	publishErr *prometheus.CounterVec
	sourceLag  *prometheus.GaugeVec
	gatherer   prometheus.Gatherer
}

func newServiceMetrics(reg prometheus.Registerer, gatherer prometheus.Gatherer) *serviceMetrics {
	if reg == nil && gatherer == nil {
		registry := prometheus.NewRegistry()
		reg = registry
		gatherer = registry
	} else if reg != nil && gatherer == nil {
		if g, ok := reg.(prometheus.Gatherer); ok {
			gatherer = g
		} else {
			gatherer = prometheus.DefaultGatherer
		}
	} else if reg == nil && gatherer != nil {
		if r, ok := gatherer.(prometheus.Registerer); ok {
			reg = r
		} else {
			registry := prometheus.NewRegistry()
			reg = registry
			gatherer = registry
		}
	}

	forwarded := promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
		Namespace: "dex",
		Subsystem: "bridge",
		Name:      observability.MetricBridgeForwardTotal,
		Help:      "Total number of messages mirrored to the legacy stream.",
	}, []string{"subject"})

	dropped := promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
		Namespace: "dex",
		Subsystem: "bridge",
		Name:      observability.MetricBridgeDroppedTotal,
		Help:      "Count of messages dropped by the bridge due to mapping rules.",
	}, []string{"subject"})

	publishErr := promauto.With(reg).NewCounterVec(prometheus.CounterOpts{
		Namespace: "dex",
		Subsystem: "bridge",
		Name:      observability.MetricBridgePublishErrors,
		Help:      "Count of publish failures when forwarding to the legacy stream.",
	}, []string{"subject"})

	sourceLag := promauto.With(reg).NewGaugeVec(prometheus.GaugeOpts{
		Namespace: "dex",
		Subsystem: "bridge",
		Name:      observability.MetricBridgeSourceLagSecond,
		Help:      "Age in seconds between the source message timestamp and bridge processing time.",
	}, []string{"subject"})

	return &serviceMetrics{
		forwarded:  forwarded,
		dropped:    dropped,
		publishErr: publishErr,
		sourceLag:  sourceLag,
		gatherer:   gatherer,
	}
}

func (m *serviceMetrics) incForwarded(subject string) {
	m.forwarded.WithLabelValues(subject).Inc()
}

func (m *serviceMetrics) incDropped(subject string) {
	m.dropped.WithLabelValues(subject).Inc()
}

func (m *serviceMetrics) incPublishError(subject string) {
	m.publishErr.WithLabelValues(subject).Inc()
}

func (m *serviceMetrics) observeLag(subject string, lag float64) {
	m.sourceLag.WithLabelValues(subject).Set(lag)
}
