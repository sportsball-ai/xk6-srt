package srt

import (
	"context"
	"fmt"
	"runtime"
	"time"

	"github.com/haivision/srtgo"
	"go.k6.io/k6/js/modules"
	"go.k6.io/k6/metrics"
)

// Register the extension on module initialization, available to
// import from JS as "k6/x/srt".
// https://k6.io/docs/extensions/getting-started/create/javascript-extensions/#use-the-advanced-module-api
func init() {
	modules.Register("k6/x/srt", &RootModule{})
}

type (
	RootModule struct{}

	ModuleInstance struct {
		srt *SRT
	}
)

var (
    _ modules.Instance = &ModuleInstance{}
    _ modules.Module   = &RootModule{}
)

func (*RootModule) NewModuleInstance(vu modules.VU) modules.Instance {
	return &ModuleInstance{srt: &SRT{vu: vu}}
}

func (mi *ModuleInstance) Exports() modules.Exports {
	return modules.Exports {
		Default: mi.srt,
	}
}

type Socket struct {
	vu modules.VU
	s *srtgo.SrtSocket
}

func countStats(vu modules.VU, before, after *srtgo.SrtStats) {
	if state := vu.State(); state != nil && before != nil && after != nil {
		ctx := vu.Context()
		now := time.Now()

		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			Metric: DataSent,
			Time:   now,
			Value:  float64(after.ByteSentTotal - before.ByteSentTotal),
		})

		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			Metric: DataReceived,
			Time:   now,
			Value:  float64(after.ByteRecvTotal - before.ByteRecvTotal),
		})

		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
			Metric: DataRetransmitted,
			Time:   now,
			Value:  float64(after.ByteRetransTotal - before.ByteRetransTotal),
		})

		metrics.PushIfNotDone(ctx, state.Samples, metrics.Sample{
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

func (srt *SRT) Stop(handle *BackgroundLoopHandle) {
	backgroundLoops[handle.Id].Stop()
}

type BackgroundLoop struct {
	cancel            func()
	done              chan struct{}
	socket            *Socket
	socketStatsBefore *srtgo.SrtStats
}

func (l *BackgroundLoop) Stop() {
	l.cancel()
	<-l.done
	socketStatsAfter := l.socket.Stats()
	countStats(l.socket.vu, l.socketStatsBefore, socketStatsAfter)
}

// Streams an MPEG-TS file up to the server in real-time.
func (s *Socket) StreamMpegtsFile(path string) bool {
	socketStatsBefore := s.Stats()
	err := StreamMPEGTSFile(s.vu.Context(), path, s.s)
	socketStatsAfter := s.Stats()

	countStats(s.vu, socketStatsBefore, socketStatsAfter)

	if err != nil {
		ReportError(fmt.Errorf("error streaming mpegts file: %w", err))
		return false
	}
	return true
}

// Streams data from the server and discards it until at least n bytes are read.
func (s *Socket) DiscardBytes(n int) bool {
	buf := make([]byte, 1316)
	socketStatsBefore := s.Stats()
	ctx := s.vu.Context()
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

	countStats(s.vu, socketStatsBefore, socketStatsAfter)

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

type SRT struct {
	vu modules.VU
}

func (srt *SRT) Connect(host string, port uint16, opts map[string]string) interface{} {
	s := srtgo.NewSrtSocket(host, port, opts)
	if s == nil {
		ReportError(fmt.Errorf("unable to create socket"))
		return nil
	}
	ret := &Socket{vu: srt.vu, s: s}
	runtime.SetFinalizer(ret, (*Socket).finalize)
	if err := ret.s.Connect(); err != nil {
			ReportError(fmt.Errorf("connection error: %w", err))
			return nil
	}
	rt := srt.vu.Runtime()
	return rt.ToValue(ret).ToObject(rt)
}
