package parquet

import (
	"context"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	proto "google.golang.org/protobuf/proto"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
)

type ServiceConfig struct {
	NATSURL     string
	Stream      string
	SubjectRoot string
	Consumer    string
	PullBatch   int
	PullTimeout time.Duration
	Writer      Config
}

func (c ServiceConfig) Validate() error {
	if c.NATSURL == "" {
		return fmt.Errorf("nats url is required")
	}
	if c.Stream == "" {
		return fmt.Errorf("nats stream is required")
	}
	if c.SubjectRoot == "" {
		return fmt.Errorf("subject root is required")
	}
	if c.Consumer == "" {
		return fmt.Errorf("consumer name is required")
	}
	if c.PullBatch <= 0 {
		return fmt.Errorf("pull batch must be positive")
	}
	if c.PullTimeout <= 0 {
		return fmt.Errorf("pull timeout must be positive")
	}
	return c.Writer.Validate()
}

func ServiceConfigFromEnv() (ServiceConfig, error) {
	cfg := ServiceConfig{
		NATSURL:     os.Getenv("PARQUET_NATS_URL"),
		Stream:      os.Getenv("PARQUET_NATS_STREAM"),
		SubjectRoot: valueOrDefault(os.Getenv("PARQUET_SUBJECT_ROOT"), "dex.sol"),
		Consumer:    valueOrDefault(os.Getenv("PARQUET_CONSUMER"), "parquet-sink"),
		PullBatch:   256,
		PullTimeout: 500 * time.Millisecond,
		Writer:      DefaultConfig(),
	}

	if v := os.Getenv("PARQUET_PULL_BATCH"); v != "" {
		if batch, err := strconv.Atoi(v); err == nil && batch > 0 {
			cfg.PullBatch = batch
		} else {
			return ServiceConfig{}, fmt.Errorf("invalid PARQUET_PULL_BATCH: %q", v)
		}
	}
	if v := os.Getenv("PARQUET_PULL_TIMEOUT_MS"); v != "" {
		ms, err := strconv.Atoi(v)
		if err != nil || ms <= 0 {
			return ServiceConfig{}, fmt.Errorf("invalid PARQUET_PULL_TIMEOUT_MS: %q", v)
		}
		cfg.PullTimeout = time.Duration(ms) * time.Millisecond
	}

	writerCfg, err := FromEnv()
	if err != nil {
		return ServiceConfig{}, err
	}
	cfg.Writer = writerCfg
	return cfg, cfg.Validate()
}

type Service struct {
	cfg       ServiceConfig
	conn      *nats.Conn
	js        nats.JetStreamContext
	sub       *nats.Subscription
	writer    *Writer
	flushTick *time.Ticker
}

func NewService(ctx context.Context, cfg ServiceConfig) (*Service, error) {
	writer, err := NewWriter(cfg.Writer)
	if err != nil {
		return nil, err
	}

	conn, err := nats.Connect(cfg.NATSURL)
	if err != nil {
		return nil, fmt.Errorf("connect nats: %w", err)
	}

	js, err := conn.JetStream()
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("jetstream: %w", err)
	}

	subject := cfg.SubjectRoot + ".candle.>"
	sub, err := js.PullSubscribe(subject, cfg.Consumer, nats.BindStream(cfg.Stream), nats.ManualAck())
	if err != nil {
		conn.Close()
		return nil, fmt.Errorf("pull subscribe: %w", err)
	}

	return &Service{
		cfg:       cfg,
		conn:      conn,
		js:        js,
		sub:       sub,
		writer:    writer,
		flushTick: time.NewTicker(cfg.Writer.FlushInterval),
	}, nil
}

func (s *Service) Run(ctx context.Context) error {
	defer s.flushTick.Stop()
	defer s.conn.Drain()
	defer s.writer.Close()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-s.flushTick.C:
			if err := s.writer.Flush(ctx); err != nil {
				return err
			}
		default:
		}

		msgs, err := s.sub.Fetch(s.cfg.PullBatch, nats.MaxWait(s.cfg.PullTimeout))
		if errors.Is(err, nats.ErrTimeout) {
			continue
		}
		if err != nil {
			return fmt.Errorf("fetch messages: %w", err)
		}

		for _, msg := range msgs {
			if err := s.handleMessage(ctx, msg); err != nil {
				_ = msg.Nak()
				return err
			}
			_ = msg.Ack()
		}
	}
}

func (s *Service) handleMessage(ctx context.Context, msg *nats.Msg) error {
	subject := msg.Subject
	if !strings.Contains(subject, ".candle.") {
		return nil
	}

	var candle dexv1.Candle
	if err := proto.Unmarshal(msg.Data, &candle); err != nil {
		return fmt.Errorf("unmarshal candle: %w", err)
	}
	return s.writer.AppendCandle(ctx, &candle)
}

func valueOrDefault(value, fallback string) string {
	if value == "" {
		return fallback
	}
	return value
}
