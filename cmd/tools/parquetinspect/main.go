package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sort"

	"github.com/parquet-go/parquet-go"
)

type candleRow struct {
	ChainID      int32  `parquet:"name=chain_id,type=INT32"`
	PairID       string `parquet:"name=pair_id,type=BYTE_ARRAY,convertedtype=UTF8"`
	PoolID       string `parquet:"name=pool_id,type=BYTE_ARRAY,convertedtype=UTF8"`
	Scope        string `parquet:"name=scope,type=BYTE_ARRAY,convertedtype=UTF8"`
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

type summary struct {
	TotalRows          int      `json:"total_rows"`
	EmptyTimeframe     int      `json:"empty_timeframe"`
	EmptyScope         int      `json:"empty_scope"`
	InvalidScope       int      `json:"invalid_scope"`
	MissingWindowStart int      `json:"missing_window_start"`
	NegativeTrades     int      `json:"negative_trades"`
	UniqueTimeframes   []string `json:"unique_timeframes"`
	UniqueScopes       []string `json:"unique_scopes"`
}

func main() {
	pattern := flag.String("pattern", "", "glob pattern selecting parquet files to inspect")
	flag.Parse()

	if *pattern == "" {
		log.Fatal("pattern is required")
	}

	files, err := filepath.Glob(*pattern)
	if err != nil {
		log.Fatalf("glob parquet files: %v", err)
	}
	if len(files) == 0 {
		log.Fatalf("no parquet files match pattern %s", *pattern)
	}

	var sum summary
	timeframeSet := make(map[string]struct{})
	scopeSet := make(map[string]struct{})

	for _, path := range files {
		if err := inspectFile(path, &sum, timeframeSet, scopeSet); err != nil {
			log.Fatalf("inspect %s: %v", path, err)
		}
	}

	sum.UniqueTimeframes = toSortedSlice(timeframeSet)
	sum.UniqueScopes = toSortedSlice(scopeSet)

	enc := json.NewEncoder(os.Stdout)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(&sum); err != nil {
		log.Fatalf("encode summary: %v", err)
	}
}

func inspectFile(path string, sum *summary, timeframeSet, scopeSet map[string]struct{}) error {
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("open file: %w", err)
	}
	defer file.Close()

	reader := parquet.NewGenericReader[candleRow](file)
	defer reader.Close()

	rows := make([]candleRow, 128)

	for {
		n, err := reader.Read(rows)
		for i := 0; i < n; i++ {
			processRow(&rows[i], sum, timeframeSet, scopeSet)
		}
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("read parquet rows: %w", err)
		}
	}
	return nil
}

func processRow(row *candleRow, sum *summary, timeframeSet, scopeSet map[string]struct{}) {
	sum.TotalRows++

	if row.Timeframe == "" {
		sum.EmptyTimeframe++
	} else {
		timeframeSet[row.Timeframe] = struct{}{}
	}

	if row.Scope == "" {
		sum.EmptyScope++
	} else {
		scopeSet[row.Scope] = struct{}{}
		if row.Scope != "pair" && row.Scope != "pool" {
			sum.InvalidScope++
		}
	}

	if row.WindowStart <= 0 {
		sum.MissingWindowStart++
	}

	if row.Trades < 0 {
		sum.NegativeTrades++
	}
}

func toSortedSlice(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for value := range set {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}
