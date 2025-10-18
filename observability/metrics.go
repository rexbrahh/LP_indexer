package observability

const (
	MetricIngestorSlotLag        = "ingestor_slot_lag"
	MetricPublisherNATSacksTotal = "publisher_nats_acks_total"
	MetricPublisherNATSErrors    = "publisher_nats_errors_total"

	MetricBridgeForwardTotal    = "bridge_forward_total"
	MetricBridgeDroppedTotal    = "bridge_dropped_total"
	MetricBridgePublishErrors   = "bridge_publish_errors_total"
	MetricBridgeSourceLagSecond = "bridge_source_lag_seconds"

	MetricRaydiumSwapsTotal   = "ingestor_raydium_swaps_total"
	MetricRaydiumDecodeErrors = "ingestor_raydium_decode_errors_total"
	MetricOrcaSwapsTotal      = "ingestor_orca_swaps_total"
	MetricOrcaDecodeErrors    = "ingestor_orca_decode_errors_total"
	MetricMeteoraSwapsTotal   = "ingestor_meteora_swaps_total"
	MetricMeteoraDecodeErrors = "ingestor_meteora_decode_errors_total"
)
