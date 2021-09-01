package srt

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/haivision/srtgo"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/lib"
	"go.k6.io/k6/stats"
)

// Register the extension on module initialization, available to
// import from JS as "k6/x/redis".
func init() {
	modules.Register("k6/x/srt", new(SRT))
}

type Socket struct {
	s *srtgo.SrtSocket
}

func (s *Socket) StreamMpegtsFile(ctx context.Context, path string) bool {
	socketStatsBefore := s.Stats()
	err := StreamMPEGTSFile(path, s.s)
	socketStatsAfter := s.Stats()

	if state := lib.GetState(ctx); state != nil && socketStatsBefore != nil && socketStatsAfter != nil {
		now := time.Now()

		stats.PushIfNotDone(ctx, state.Samples, stats.Sample{
			Metric: DataSent,
			Time:   now,
			Value:  float64(socketStatsAfter.ByteSentTotal - socketStatsBefore.ByteSentTotal),
		})

		stats.PushIfNotDone(ctx, state.Samples, stats.Sample{
			Metric: DataRetransmitted,
			Time:   now,
			Value:  float64(socketStatsAfter.ByteRetransTotal - socketStatsBefore.ByteRetransTotal),
		})
	}

	if err != nil {
		ReportError(fmt.Errorf("error streaming mpegts file: %w", err))
		return false
	}
	return true
}

func (s *Socket) Stats() *srtgo.SrtStats {
	if stats, err := s.s.Stats(); err != nil {
		ReportError(fmt.Errorf("error getting stats: %w", err))
		return nil
	} else {
		return stats
	}
}

func (s *Socket) finalize() {
	s.s.Close()
}

type SRT struct{}

func (*SRT) Connect(ctxPtr *context.Context, host string, port uint16, opts map[string]string) interface{} {
	s := srtgo.NewSrtSocket(host, port, opts)
	if s == nil {
		ReportError(fmt.Errorf("unable to create socket"))
		return nil
	}
	ret := &Socket{s: s}
	runtime.SetFinalizer(ret, (*Socket).finalize)
	if err := ret.s.Connect(); err != nil {
		ReportError(fmt.Errorf("connection error: %w", err))
		return nil
	}
	rt := common.GetRuntime(*ctxPtr)
	return common.Bind(rt, ret, ctxPtr)
}
