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

func countStats(ctx context.Context, before, after *srtgo.SrtStats) {
	if state := lib.GetState(ctx); state != nil && before != nil && after != nil {
		now := time.Now()

		stats.PushIfNotDone(ctx, state.Samples, stats.Sample{
			Metric: DataSent,
			Time:   now,
			Value:  float64(after.ByteSentTotal - before.ByteSentTotal),
		})

		stats.PushIfNotDone(ctx, state.Samples, stats.Sample{
			Metric: DataReceived,
			Time:   now,
			Value:  float64(after.ByteRecvTotal - before.ByteRecvTotal),
		})

		stats.PushIfNotDone(ctx, state.Samples, stats.Sample{
			Metric: DataRetransmitted,
			Time:   now,
			Value:  float64(after.ByteRetransTotal - before.ByteRetransTotal),
		})

		stats.PushIfNotDone(ctx, state.Samples, stats.Sample{
			Metric: DataReceiveLoss,
			Time:   now,
			Value:  float64(after.ByteRcvLossTotal - before.ByteRcvLossTotal),
		})
	}

}

var backgroundLoops = map[int]*BackgroundLoop{}

// Starts streaming an MPEG-TS file in the background. To stop it, pass the returned object to
// Stop().
func (s *Socket) StartMpegtsFileBackgroundLoop(path string) *BackgroundLoopHandle {
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	socketStatsBefore := s.Stats()
	go func() {
		defer close(done)
		for {
			err := StreamMPEGTSFile(ctx, path, s.s)

			if err != nil {
				if ctx.Err() != nil {
					return
				}
				ReportError(fmt.Errorf("error streaming mpegts file: %w", err))
				time.Sleep(time.Second)
			}
		}
	}()
	id := len(backgroundLoops)
	backgroundLoops[id] = &BackgroundLoop{
		cancel:            cancel,
		done:              done,
		socket:            s,
		socketStatsBefore: socketStatsBefore,
	}
	return &BackgroundLoopHandle{
		Id: id,
	}
}

type BackgroundLoopHandle struct {
	Id int
}

func (*SRT) Stop(ctx context.Context, handle *BackgroundLoopHandle) {
	backgroundLoops[handle.Id].Stop(ctx)
}

type BackgroundLoop struct {
	cancel            func()
	done              chan struct{}
	socket            *Socket
	socketStatsBefore *srtgo.SrtStats
}

func (l *BackgroundLoop) Stop(ctx context.Context) {
	l.cancel()
	<-l.done
	socketStatsAfter := l.socket.Stats()
	countStats(ctx, l.socketStatsBefore, socketStatsAfter)
}

// Streams an MPEG-TS file up to the server in real-time.
func (s *Socket) StreamMpegtsFile(ctx context.Context, path string) bool {
	socketStatsBefore := s.Stats()
	err := StreamMPEGTSFile(ctx, path, s.s)
	socketStatsAfter := s.Stats()

	countStats(ctx, socketStatsBefore, socketStatsAfter)

	if err != nil {
		ReportError(fmt.Errorf("error streaming mpegts file: %w", err))
		return false
	}
	return true
}

// Streams data from the server and discards it until at least n bytes are read.
func (s *Socket) DiscardBytes(ctx context.Context, n int) bool {
	buf := make([]byte, 1316)
	socketStatsBefore := s.Stats()
	var err error
	for n > 0 && err == nil {
		err = ctx.Err()
		if err == nil {
			if bytesRead, readErr := s.s.Read(buf); readErr == nil {
				n -= bytesRead
			} else {
				err = readErr
			}
		}
	}
	socketStatsAfter := s.Stats()

	countStats(ctx, socketStatsBefore, socketStatsAfter)

	if err != nil {
		ReportError(fmt.Errorf("error streaming bytes: %w", err))
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

func (s *Socket) Close() {
	s.s.Close()
}

func (s *Socket) finalize() {
	s.Close()
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
