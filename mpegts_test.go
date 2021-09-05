package srt

import (
	"context"
	"testing"

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
	require.NoError(t, StreamMPEGTSFile(context.Background(), "testdata/testsrc.ts", w))
	assert.Equal(t, w.bytesReceived, 438792)
}
