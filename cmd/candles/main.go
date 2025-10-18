package main

import (
    "context"
    "errors"
    "flag"
    "log"
    "os"
    "os/signal"
    "strconv"
    "syscall"
    "time"

    nats "github.com/nats-io/nats.go"
    "google.golang.org/protobuf/proto"

    dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
    "github.com/rexbrahh/lp-indexer/sinks/clickhouse"
)

func main() {
    var (
        natsURL    = flag.String("nats", envOr("NATS_URL", "nats://127.0.0.1:4222"), "NATS connection URL")
        stream     = flag.String("stream", envOr("NATS_STREAM", "DEX"), "JetStream stream name")
        subject    = flag.String("subject", envOr("NATS_CANDLE_SUBJECT", "dex.sol.candle.>"), "Candle subject filter")
        durable    = flag.String("durable", envOr("NATS_DURABLE", "candle_bridge"), "Durable consumer name")
        batchSize  = flag.Int("batch", envOrInt("BATCH", 64), "JetStream pull batch size")
        pullWaitMs = flag.Int("pull-wait", envOrInt("PULL_WAIT_MS", 500), "Pull wait in milliseconds")

        clickhouseDSN      = flag.String("clickhouse-dsn", envOr("CLICKHOUSE_DSN", "tcp://127.0.0.1:9000"), "ClickHouse DSN")
        clickhouseDatabase = flag.String("clickhouse-db", envOr("CLICKHOUSE_DB", "default"), "ClickHouse database")
        clickhouseCandles  = flag.String("clickhouse-candles", envOr("CLICKHOUSE_CANDLES_TABLE", "candles"), "Candles table")
    )
    flag.Parse()

    logger := log.New(os.Stdout, "candles-bridge ", log.LstdFlags|log.Lshortfile)

    opts := []nats.Option{nats.Name("candles-bridge")}
    conn, err := nats.Connect(*natsURL, opts...)
    if err != nil {
        logger.Fatalf("connect NATS: %v", err)
    }
    defer conn.Drain()

    js, err := conn.JetStream()
    if err != nil {
        logger.Fatalf("jetstream: %v", err)
    }

    pullOpts := []nats.SubOpt{nats.BindStream(*stream)}
    sub, err := js.PullSubscribe(*subject, *durable, pullOpts...)
    if err != nil {
        logger.Fatalf("pull subscribe: %v", err)
    }

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    go func() {
        <-sigCh
        logger.Println("shutdown signal received")
        cancel()
    }()

    writerCfg := clickhouse.Config{
        DSN:          *clickhouseDSN,
        Database:     *clickhouseDatabase,
        TradesTable:  envOr("CLICKHOUSE_TRADES_TABLE", "trades"),
        CandlesTable: *clickhouseCandles,
        BatchSize:    envOrInt("CLICKHOUSE_CANDLES_BATCH", 512),
    }

    writer, err := clickhouse.NewWithConfig(ctx, writerCfg)
    if err != nil {
        logger.Fatalf("init ClickHouse writer: %v", err)
    }
    defer func() {
        // No explicit close currently, placeholder for future cleanup.
        _ = writer
    }()

    bridge := &Bridge{
        logger: logger,
        writer: writer,
    }

    wait := time.Duration(*pullWaitMs) * time.Millisecond

    for {
        select {
        case <-ctx.Done():
            return
        default:
        }

        msgs, err := sub.Fetch(*batchSize, nats.MaxWait(wait))
        if err != nil {
            if errors.Is(err, nats.ErrTimeout) {
                continue
            }
            logger.Printf("fetch error: %v", err)
            time.Sleep(500 * time.Millisecond)
            continue
        }

        if err := bridge.Process(ctx, msgs); err != nil {
            logger.Printf("process batch: %v", err)
        }
    }
}

type Bridge struct {
    logger *log.Logger
    writer *clickhouse.Writer
}

func (b *Bridge) Process(ctx context.Context, msgs []*nats.Msg) error {
    candles := make([]clickhouse.Candle, 0, len(msgs))

    for _, msg := range msgs {
        var candle dexv1.Candle
        if err := proto.Unmarshal(msg.Data, &candle); err != nil {
            b.logger.Printf("decode candle: %v", err)
            _ = msg.Nak()
            continue
        }

        candles = append(candles, translateCandle(&candle))

        if err := msg.Ack(); err != nil {
            b.logger.Printf("ack failed: %v", err)
        }
    }

    if len(candles) == 0 {
        return nil
    }

    if err := b.writer.WriteCandles(ctx, candles); err != nil {
        return err
    }

    return nil
}

func translateCandle(c *dexv1.Candle) clickhouse.Candle {
    return clickhouse.Candle{
        Timestamp: time.Unix(int64(c.GetWindowStart()), 0).UTC(),
        PoolID:    c.GetPoolId(),
        Open:      float64(c.GetOpenPxQ32()),
        High:      float64(c.GetHighPxQ32()),
        Low:       float64(c.GetLowPxQ32()),
        Close:     float64(c.GetClosePxQ32()),
        Volume:    float64(c.GetVolQuote().GetLo()),
    }
}

func envOr(key, fallback string) string {
    if v := os.Getenv(key); v != "" {
        return v
    }
    return fallback
}

func envOrInt(key string, fallback int) int {
    if v := os.Getenv(key); v != "" {
        if iv, err := strconv.Atoi(v); err == nil {
            return iv
        }
    }
    return fallback
}
