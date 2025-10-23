package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/nats-io/nats.go"
	proto "google.golang.org/protobuf/proto"

	dexv1 "github.com/rexbrahh/lp-indexer/gen/go/dex/sol/v1"
)

type event struct {
	Type          string   `json:"type"`
	Slot          uint64   `json:"slot"`
	Timestamp     int64    `json:"timestamp"`
	Status        string   `json:"status"`
	Signature     string   `json:"signature"`
	Index         uint32   `json:"index"`
	ProgramID     string   `json:"program_id"`
	PoolID        string   `json:"pool_id"`
	MintBase      string   `json:"mint_base"`
	MintQuote     string   `json:"mint_quote"`
	DecBase       uint32   `json:"dec_base"`
	DecQuote      uint32   `json:"dec_quote"`
	BaseIn        uint64   `json:"base_in"`
	BaseOut       uint64   `json:"base_out"`
	QuoteIn       uint64   `json:"quote_in"`
	QuoteOut      uint64   `json:"quote_out"`
	ReservesBase  uint64   `json:"reserves_base"`
	ReservesQuote uint64   `json:"reserves_quote"`
	FeeBps        uint32   `json:"fee_bps"`
	Provisional   *bool    `json:"provisional"`
	IsUndo        bool     `json:"is_undo"`
	Success       *bool    `json:"success"`
	LogMsgs       []string `json:"log_msgs"`
	SleepMillis   int      `json:"sleep_ms"`
}

func main() {
	inputPath := flag.String("input", "fixtures/sink_sample.json", "path to event fixture (JSON)")
	natsURL := flag.String("nats-url", "nats://127.0.0.1:4222", "NATS server URL")
	subjectRoot := flag.String("subject-root", "dex.sol", "subject root for publishing")
	publishDelay := flag.Int("delay-ms", 0, "delay in milliseconds between events")
	flag.Parse()

	data, err := os.ReadFile(*inputPath)
	if err != nil {
		log.Fatalf("failed to read input: %v", err)
	}

	var events []event
	if err := json.Unmarshal(data, &events); err != nil {
		log.Fatalf("failed to decode fixture: %v", err)
	}

	nc, err := nats.Connect(*natsURL)
	if err != nil {
		log.Fatalf("connect to nats: %v", err)
	}
	defer nc.Drain()

	js, err := nc.JetStream()
	if err != nil {
		log.Fatalf("jetstream context: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	for idx, ev := range events {
		if ctx.Err() != nil {
			log.Fatalf("context cancelled before event %d", idx)
		}
		if err := publishEvent(ctx, js, *subjectRoot, ev); err != nil {
			log.Fatalf("failed to publish event %d (%s): %v", idx, ev.Type, err)
		}
		delay := ev.SleepMillis
		if delay == 0 {
			delay = *publishDelay
		}
		if delay > 0 {
			time.Sleep(time.Duration(delay) * time.Millisecond)
		}
	}

	log.Printf("published %d events", len(events))
}

func publishEvent(ctx context.Context, js nats.JetStreamContext, root string, ev event) error {
	switch ev.Type {
	case "block_head":
		msg := &dexv1.BlockHead{
			ChainId: 501,
			Slot:    ev.Slot,
			TsSec:   uint64(ev.Timestamp),
			Status:  ev.Status,
		}
		data, err := proto.Marshal(msg)
		if err != nil {
			return err
		}
		return publishProto(ctx, js, fmt.Sprintf("%s.blocks.head", root), data, fmt.Sprintf("501:%d:head:%s", ev.Slot, ev.Status))
	case "tx_meta":
		success := true
		if ev.Success != nil {
			success = *ev.Success
		}
		msg := &dexv1.TxMeta{
			ChainId: 501,
			Slot:    ev.Slot,
			Sig:     ev.Signature,
			Success: success,
			CuUsed:  0,
			CuPrice: 0,
			LogMsgs: ev.LogMsgs,
		}
		data, err := proto.Marshal(msg)
		if err != nil {
			return err
		}
		return publishProto(ctx, js, fmt.Sprintf("%s.tx.meta", root), data, fmt.Sprintf("501:%d:%s:meta", ev.Slot, ev.Signature))
	case "swap":
		provisional := true
		if ev.Provisional != nil {
			provisional = *ev.Provisional
		}
		msg := &dexv1.SwapEvent{
			ChainId:       501,
			Slot:          ev.Slot,
			Sig:           ev.Signature,
			Index:         ev.Index,
			ProgramId:     ev.ProgramID,
			PoolId:        ev.PoolID,
			MintBase:      ev.MintBase,
			MintQuote:     ev.MintQuote,
			DecBase:       ev.DecBase,
			DecQuote:      ev.DecQuote,
			BaseIn:        ev.BaseIn,
			BaseOut:       ev.BaseOut,
			QuoteIn:       ev.QuoteIn,
			QuoteOut:      ev.QuoteOut,
			ReservesBase:  ev.ReservesBase,
			ReservesQuote: ev.ReservesQuote,
			FeeBps:        ev.FeeBps,
			Provisional:   provisional,
			IsUndo:        ev.IsUndo,
		}
		data, err := proto.Marshal(msg)
		if err != nil {
			return err
		}
		subject := fmt.Sprintf("%s.%s.swap", root, programSegment(ev.ProgramID))
		return publishProto(ctx, js, subject, data, fmt.Sprintf("501:%d:%s:%d:%t:%t", ev.Slot, ev.Signature, ev.Index, provisional, ev.IsUndo))
	default:
		return fmt.Errorf("unsupported event type %q", ev.Type)
	}
}

func programSegment(programID string) string {
	if programID == "" {
		return "unknown"
	}
	switch programID {
	case "CAMMCzo5YL8w4VFF8KVHrK22GGUsp5VTaW7grrKgrWqK":
		return "raydium"
	case "whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc":
		return "orca"
	case "cpamdpZCGKUy5JxQXB4dcpGPiikHawvSWAd6mEn1sGG":
		return "meteora"
	default:
		cleaned := programID
		if len(cleaned) > 12 {
			cleaned = cleaned[:12]
		}
		return cleaned
	}
}

func publishProto(ctx context.Context, js nats.JetStreamContext, subject string, data []byte, msgID string) error {
	msg := &nats.Msg{Subject: subject, Data: data}
	msg.Header = nats.Header{}
	if msgID != "" {
		msg.Header.Set("Nats-Msg-Id", msgID)
	}
	msg.Header.Set("Content-Type", "application/protobuf")
	_, err := js.PublishMsgAsync(msg)
	if err != nil {
		return err
	}
	select {
	case <-js.PublishAsyncComplete():
		return nil
	case <-ctx.Done():
		return fmt.Errorf("publish context cancelled for subject %s", subject)
	case <-time.After(3 * time.Second):
		return fmt.Errorf("publish timeout for subject %s", subject)
	}
}
