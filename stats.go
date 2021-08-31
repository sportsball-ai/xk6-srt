package srt

import "go.k6.io/k6/stats"

var (
	DataSent          = stats.New("srt_data_sent", stats.Counter, stats.Data)
	DataRetransmitted = stats.New("srt_data_retransmitted", stats.Counter, stats.Data)
)
