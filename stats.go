package srt

import "go.k6.io/k6/metrics"

var (
	registry          = metrics.NewRegistry()
	DataSent          = registry.MustNewMetric("srt_data_sent", metrics.Counter, metrics.Data)
	DataReceived      = registry.MustNewMetric("srt_data_received", metrics.Counter, metrics.Data)
	DataRetransmitted = registry.MustNewMetric("srt_data_retransmitted", metrics.Counter, metrics.Data)
	DataReceiveLoss   = registry.MustNewMetric("srt_data_receive_loss", metrics.Counter, metrics.Data)
)
