package srt

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testWriter struct {
	t             *testing.T
	gotLast       bool
	bytesReceived int
}

func (w *testWriter) Write(buf []byte) (int, error) {
	if !w.gotLast && len(buf) != 1316 {
		w.gotLast = true
		assert.Equal(w.t, len(buf)%188, 0)
	} else {
		assert.Len(w.t, buf, 1316)
	}
	w.bytesReceived += len(buf)
	return len(buf), nil
}

func TestStreamMPEGTSFile(t *testing.T) {
	w := &testWriter{
		t: t,
	}
	startTime := time.Now()
	require.NoError(t, StreamMPEGTSFile(context.Background(), "testdata/testsrc.ts", w))
	d := time.Since(startTime)
	assert.Greater(t, d, 5500*time.Millisecond)
	assert.Equal(t, w.bytesReceived, 438792)
}
