#!/usr/bin/env bash
# jetstream-validate.sh
# Displays JetStream stream and consumer validation details for quick inspection.
#
# Usage:
#   ./scripts/jetstream-validate.sh

set -euo pipefail

# Check nats CLI is installed
if ! command -v nats >/dev/null 2>&1; then
    echo "ERROR: nats CLI not found. Install with: brew install nats-io/nats-tools/nats"
    exit 1
fi

echo "=========================================="
echo "JetStream Validation Report"
echo "=========================================="
echo ""

# Stream DEX validation
echo "Stream: DEX"
echo "----------------------------------------"
if nats stream info DEX >/dev/null 2>&1; then
    echo "✓ Stream exists"

    # Extract key configuration details
    RETENTION=$(nats stream info DEX -j | jq -r '.config.retention')
    REPLICAS=$(nats stream info DEX -j | jq -r '.config.num_replicas')
    DUPLICATE_WINDOW=$(nats stream info DEX -j | jq -r '.config.duplicate_window')
    STORAGE=$(nats stream info DEX -j | jq -r '.config.storage')
    MESSAGES=$(nats stream info DEX -j | jq -r '.state.messages')
    BYTES=$(nats stream info DEX -j | jq -r '.state.bytes')

    echo "  Retention:        $RETENTION"
    echo "  Replicas:         $REPLICAS"
    echo "  Duplicate Window: $DUPLICATE_WINDOW"
    echo "  Storage:          $STORAGE"
    echo "  Messages:         $MESSAGES"
    echo "  Bytes:            $BYTES"
else
    echo "✗ Stream does not exist"
    exit 1
fi

echo ""
echo "Consumer: SWAP_FIREHOSE"
echo "----------------------------------------"
if nats consumer info DEX SWAP_FIREHOSE >/dev/null 2>&1; then
    echo "✓ Consumer exists"

    # Extract key consumer details
    ACK_POLICY=$(nats consumer info DEX SWAP_FIREHOSE -j | jq -r '.config.ack_policy')
    DELIVER_POLICY=$(nats consumer info DEX SWAP_FIREHOSE -j | jq -r '.config.deliver_policy')
    FILTER_SUBJECT=$(nats consumer info DEX SWAP_FIREHOSE -j | jq -r '.config.filter_subject')
    MAX_DELIVER=$(nats consumer info DEX SWAP_FIREHOSE -j | jq -r '.config.max_deliver')
    MAX_ACK_PENDING=$(nats consumer info DEX SWAP_FIREHOSE -j | jq -r '.config.max_ack_pending')
    ACK_PENDING=$(nats consumer info DEX SWAP_FIREHOSE -j | jq -r '.num_ack_pending')
    NUM_PENDING=$(nats consumer info DEX SWAP_FIREHOSE -j | jq -r '.num_pending')
    REDELIVERED=$(nats consumer info DEX SWAP_FIREHOSE -j | jq -r '.num_redelivered')

    echo "  Ack Policy:       $ACK_POLICY"
    echo "  Deliver Policy:   $DELIVER_POLICY"
    echo "  Filter Subject:   $FILTER_SUBJECT"
    echo "  Max Deliver:      $MAX_DELIVER"
    echo "  Max Ack Pending:  $MAX_ACK_PENDING"
    echo "  Ack Pending:      $ACK_PENDING"
    echo "  Num Pending:      $NUM_PENDING"
    echo "  Redelivered:      $REDELIVERED"
else
    echo "✗ Consumer does not exist"
    exit 1
fi

echo ""
echo "=========================================="
echo "✓ All JetStream components validated"
echo "=========================================="
