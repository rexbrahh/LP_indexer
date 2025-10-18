package parquet

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/credentials"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3/s3manager"
	"github.com/parquet-go/parquet-go"
	"github.com/parquet-go/parquet-go/compress/snappy"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
)

var ErrWriterDisabled = errors.New("parquet writer disabled: missing configuration")

// Writer buffers candles and periodically uploads Parquet files to S3-compatible storage.
type Writer struct {
	cfg Config

	mu        sync.Mutex
	buckets   map[string][]candleRow
	uploader  *s3manager.Uploader
	lastFlush time.Time
}

type candleRow struct {
	ChainID      int32  `parquet:"name=chain_id,type=INT32"`
	PairID       string `parquet:"name=pair_id,type=BYTE_ARRAY,convertedtype=UTF8"`
	PoolID       string `parquet:"name=pool_id,type=BYTE_ARRAY,convertedtype=UTF8"`
	Timeframe    string `parquet:"name=timeframe,type=BYTE_ARRAY,convertedtype=UTF8"`
	WindowStart  int64  `parquet:"name=window_start,type=INT64,logicaltype=TIMESTAMP(isAdjustedToUTC=true,unit=SECONDS)"`
	Provisional  bool   `parquet:"name=provisional,type=BOOLEAN"`
	IsCorrection bool   `parquet:"name=is_correction,type=BOOLEAN"`
	OpenPxQ32    int64  `parquet:"name=open_px_q32,type=INT64"`
	HighPxQ32    int64  `parquet:"name=high_px_q32,type=INT64"`
	LowPxQ32     int64  `parquet:"name=low_px_q32,type=INT64"`
	ClosePxQ32   int64  `parquet:"name=close_px_q32,type=INT64"`
	VolBaseHi    uint64 `parquet:"name=vol_base_hi,type=INT64"`
	VolBaseLo    uint64 `parquet:"name=vol_base_lo,type=INT64"`
	VolQuoteHi   uint64 `parquet:"name=vol_quote_hi,type=INT64"`
	VolQuoteLo   uint64 `parquet:"name=vol_quote_lo,type=INT64"`
	Trades       int32  `parquet:"name=trades,type=INT32"`
}

// NewWriter validates configuration and prepares a Writer.
func NewWriter(cfg Config) (*Writer, error) {
	if cfg.Endpoint == "" || cfg.Bucket == "" || cfg.AccessKey == "" || cfg.SecretKey == "" {
		return nil, ErrWriterDisabled
	}
	if err := cfg.Validate(); err != nil {
		return nil, err
	}

	awsCfg := &aws.Config{
		Endpoint:         aws.String(cfg.Endpoint),
		Region:           aws.String(cfg.Region),
		S3ForcePathStyle: aws.Bool(true),
		Credentials:      credentials.NewStaticCredentials(cfg.AccessKey, cfg.SecretKey, ""),
	}

	sess, err := session.NewSession(awsCfg)
	if err != nil {
		return nil, fmt.Errorf("create aws session: %w", err)
	}

	return &Writer{
		cfg:       cfg,
		buckets:   make(map[string][]candleRow),
		uploader:  s3manager.NewUploader(sess),
		lastFlush: time.Now(),
	}, nil
}

func (w *Writer) AppendSwap(ctx context.Context, event *dexv1.SwapEvent) error {
	_ = ctx
	_ = event
	return nil
}

func (w *Writer) AppendCandle(ctx context.Context, candle *dexv1.Candle) error {
	if candle == nil {
		return errors.New("nil candle")
	}

	w.mu.Lock()
	defer w.mu.Unlock()

	timeframe := candle.GetTimeframe()
	if timeframe == "" {
		timeframe = "unknown"
	}

	row := candleRow{
		ChainID:      int32(candle.GetChainId()),
		PairID:       candle.GetPairId(),
		PoolID:       candle.GetPoolId(),
		Timeframe:    timeframe,
		WindowStart:  int64(candle.GetWindowStart()),
		Provisional:  candle.GetProvisional(),
		IsCorrection: candle.GetIsCorrection(),
		OpenPxQ32:    candle.GetOpenPxQ32(),
		HighPxQ32:    candle.GetHighPxQ32(),
		LowPxQ32:     candle.GetLowPxQ32(),
		ClosePxQ32:   candle.GetClosePxQ32(),
		Trades:       int32(candle.GetTrades()),
	}

	if vb := candle.GetVolBase(); vb != nil {
		row.VolBaseHi = vb.GetHi()
		row.VolBaseLo = vb.GetLo()
	}
	if vq := candle.GetVolQuote(); vq != nil {
		row.VolQuoteHi = vq.GetHi()
		row.VolQuoteLo = vq.GetLo()
	}

	bucket := append(w.buckets[timeframe], row)
	w.buckets[timeframe] = bucket

	if len(bucket) >= w.cfg.BatchRows || time.Since(w.lastFlush) >= w.cfg.FlushInterval {
		return w.flushLocked(ctx)
	}
	return nil
}

func (w *Writer) Flush(ctx context.Context) error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.flushLocked(ctx)
}

func (w *Writer) Close() error {
	return w.Flush(context.Background())
}

func (w *Writer) flushLocked(ctx context.Context) error {
	if len(w.buckets) == 0 {
		return nil
	}

	for timeframe, rows := range w.buckets {
		if len(rows) == 0 {
			continue
		}
		if err := w.writeBucket(ctx, timeframe, rows); err != nil {
			return err
		}
		w.buckets[timeframe] = w.buckets[timeframe][:0]
	}
	w.lastFlush = time.Now()
	return nil
}

func (w *Writer) writeBucket(ctx context.Context, timeframe string, rows []candleRow) error {
	buf := bytes.NewBuffer(nil)

	writer := parquet.NewGenericWriter[candleRow](buf, parquet.Compression(&snappy.Codec{}))
	if _, err := writer.Write(rows); err != nil {
		return fmt.Errorf("write parquet rows: %w", err)
	}
	if err := writer.Close(); err != nil {
		return fmt.Errorf("close parquet writer: %w", err)
	}

	key := w.objectKey(timeframe)

	_, err := w.uploader.UploadWithContext(ctx, &s3manager.UploadInput{
		Bucket:      aws.String(w.cfg.Bucket),
		Key:         aws.String(key),
		Body:        bytes.NewReader(buf.Bytes()),
		ContentType: aws.String("application/octet-stream"),
	})
	if err != nil {
		return fmt.Errorf("upload parquet to s3: %w", err)
	}
	return nil
}

func (w *Writer) objectKey(timeframe string) string {
	prefix := strings.TrimSuffix(w.cfg.Prefix, "/")
	date := time.Now().UTC().Format("2006-01-02")
	filename := fmt.Sprintf("candles-%d.parquet", time.Now().UnixNano())
	return filepath.Join(prefix, fmt.Sprintf("timeframe=%s", timeframe), fmt.Sprintf("date=%s", date), filename)
}
