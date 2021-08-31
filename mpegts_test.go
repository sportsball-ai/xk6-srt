package srt

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type testWriter struct {
	t             *testing.T
	bytesReceived int
}

func (w *testWriter) Write(buf []byte) (int, error) {
	require.Len(w.t, buf, 188)
	w.bytesReceived += len(buf)
	return len(buf), nil
}

func TestStreamMPEGTSFile(t *testing.T) {
	w := &testWriter{
		t: t,
	}
	require.NoError(t, StreamMPEGTSFile("testdata/testsrc.ts", w))
	assert.Equal(t, w.bytesReceived, 438792)
}
