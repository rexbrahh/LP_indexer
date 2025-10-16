package observability

const (
	MetricIngestorSlotLag        = "ingestor_slot_lag"
	MetricPublisherNATSacksTotal = "publisher_nats_acks_total"
	MetricPublisherNATSErrors    = "publisher_nats_errors_total"

	MetricBridgeForwardTotal    = "bridge_forward_total"
	MetricBridgeDroppedTotal    = "bridge_dropped_total"
	MetricBridgePublishErrors   = "bridge_publish_errors_total"
	MetricBridgeSourceLagSecond = "bridge_source_lag_seconds"
)
