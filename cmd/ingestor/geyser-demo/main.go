package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"github.com/rexbrahh/lp-indexer/ingestor/geyser"
	pb "github.com/rpcpool/yellowstone-grpc/examples/golang/proto"
)

func main() {
	// Load configuration from environment variables
	cfg := &geyser.Config{
		Endpoint: os.Getenv("GEYSER_ENDPOINT"),
		APIKey:   os.Getenv("GEYSER_API_KEY"),
		ProgramFilters: map[string]string{
			// For demo purposes, subscribe to a small set of programs
			// In production, these would come from ops/programs.yaml
			"raydium_amm":     "675kPX9MHTjS2zt1qfr1NYHuzeLXfQM9H24wFSUt1Mp8",
			"orca_whirlpool":  "whirLbMiicVdio4qvUfM5KAg6Ct8VwpYzGff3uctyCc",
			"meteora_pools":   "LBUZKhRxPF3XUpBCjp4YzTKgLccjZhTSDM9YuVaPwxo",
		},
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		log.Fatalf("Invalid configuration: %v\n\nRequired environment variables:\n  GEYSER_ENDPOINT (e.g., solana-mainnet.core.chainstack.com:443)\n  GEYSER_API_KEY (your Chainstack API key)\n", err)
	}

	log.Printf("Connecting to Geyser endpoint: %s", cfg.Endpoint)

	// Create and connect client
	client, err := geyser.NewClient(cfg)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("failed to close geyser client: %v", err)
		}
	}()

	if err := client.Connect(); err != nil {
		log.Fatalf("Failed to connect: %v", err)
	}

	log.Println("Connected successfully! Starting subscription...")

	// Subscribe starting from slot 0 (will start from latest)
	updateCh, errCh := client.Subscribe(0)

	// Set up signal handling for graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Process updates
	slotCount := 0
	for {
		select {
		case update := <-updateCh:
			if update == nil {
				log.Println("Update channel closed")
				return
			}

			// Log slot updates to stdout
			switch u := update.UpdateOneof.(type) {
			case *pb.SubscribeUpdate_Slot:
				slotCount++
				log.Printf("Slot %d (parent: %d) - Total slots received: %d",
					u.Slot.Slot,
					u.Slot.Parent,
					slotCount)

			case *pb.SubscribeUpdate_Account:
				log.Printf("Account update at slot %d: %s (owner: %s)",
					u.Account.Slot,
					truncateBase58(u.Account.Account.Pubkey),
					truncateBase58(u.Account.Account.Owner))

			case *pb.SubscribeUpdate_BlockMeta:
				log.Printf("Block metadata at slot %d: block time %d",
					u.BlockMeta.Slot,
					u.BlockMeta.BlockTime.Timestamp)

			case *pb.SubscribeUpdate_Ping:
				// Respond to ping to keep connection alive
				log.Println("Received ping from server")
			}

		case err := <-errCh:
			if err == nil {
				log.Println("Error channel closed")
				return
			}
			log.Printf("Stream error (will reconnect): %v", err)

		case <-sigCh:
			log.Println("\nReceived shutdown signal, closing connection...")
			return
		}
	}
}

// truncateBase58 truncates a base58-encoded pubkey for display
func truncateBase58(pubkey []byte) string {
	if len(pubkey) == 0 {
		return "<empty>"
	}
	// Convert to base58 representation (simplified for demo)
	s := fmt.Sprintf("%x", pubkey)
	if len(s) > 16 {
		return s[:8] + "..." + s[len(s)-8:]
	}
	return s
}
