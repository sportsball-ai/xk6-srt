package srt

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

func StreamMPEGTSFile(ctx context.Context, path string, w io.Writer) error {
	f, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("error opening file: %w", err)
	}
	defer f.Close()

	var startPCR time.Duration
	startTime := time.Now()
	lastPCR := time.Duration(0)
	buf := make([]byte, 1316)

	for {
		if err := ctx.Err(); err != nil {
			return ctx.Err()
		} else if n, err := io.ReadFull(f, buf); err == io.EOF {
			return nil
		} else if err == io.ErrUnexpectedEOF {
			buf = buf[:n]
		} else if err != nil {
			return fmt.Errorf("error reading file: %w", err)
		}

		p := buf[len(buf)-188:]
		if p[0] != 0x47 {
			return fmt.Errorf("incorrect sync byte")
		}

		pcr := lastPCR
		if p[3]&0x20 != 0 {
			adaptationFieldLength := p[4]
			if adaptationFieldLength > 183 {
				return fmt.Errorf("adaptation field length is too long")
			} else if adaptationFieldLength >= 7 && p[5]&0x10 != 0 {
				base := uint64(p[6])<<25 | uint64(p[7])<<17 | uint64(p[8])<<9 | uint64(p[9])<<1 | uint64(p[10])>>7
				ext := uint64(p[10])&0x80<<1 | uint64(p[11])
				pcr27mhz := base*300 + ext
				pcr = time.Duration(pcr27mhz/27) * time.Microsecond
			}
		}

		now := time.Now()
		if pcr-lastPCR > time.Minute || pcr-lastPCR < -500*time.Millisecond {
			startPCR = pcr
			startTime = now
		}
		if packetTime := startTime.Add(pcr - startPCR); now.Before(packetTime) {
			time.Sleep(packetTime.Sub(now))
		}
		lastPCR = pcr

		if _, err := w.Write(buf); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}
}
