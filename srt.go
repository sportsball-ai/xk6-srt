package srt

import (
	"context"
	"fmt"
	"runtime"

	"github.com/haivision/srtgo"
	"go.k6.io/k6/js/common"
	"go.k6.io/k6/js/modules"
)

// Register the extension on module initialization, available to
// import from JS as "k6/x/redis".
func init() {
	modules.Register("k6/x/srt", new(SRT))
}

type Socket struct {
	s *srtgo.SrtSocket
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
	s := &Socket{s: srtgo.NewSrtSocket(host, port, opts)}
	runtime.SetFinalizer(s, (*Socket).finalize)
	if err := s.s.Connect(); err != nil {
		ReportError(fmt.Errorf("connection error: %w", err))
		return nil
	}
	rt := common.GetRuntime(*ctxPtr)
	return common.Bind(rt, s, ctxPtr)
}
