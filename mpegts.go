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
	lastPCR := time.Duration(-1)
	buf := make([]byte, 188)

	for {
		if err := ctx.Err(); err != nil {
			return ctx.Err()
		} else if _, err := io.ReadFull(f, buf); err == io.EOF {
			return nil
		} else if err != nil {
			return fmt.Errorf("error reading file: %w", err)
		}
		if buf[0] != 0x47 {
			return fmt.Errorf("incorrect sync byte")
		}

		pcr := lastPCR
		if buf[3]&0x20 != 0 {
			adaptationFieldLength := buf[4]
			if adaptationFieldLength > 183 {
				return fmt.Errorf("adaptation field length is too long")
			} else if adaptationFieldLength >= 7 && buf[5]&0x10 != 0 {
				base := uint64(buf[6])<<25 | uint64(buf[7])<<17 | uint64(buf[8])<<9 | uint64(buf[9])<<1 | uint64(buf[10])>>7
				ext := uint64(buf[10])&0x80<<1 | uint64(buf[11])
				pcr27mhz := base*300 + ext
				pcr = time.Duration(pcr27mhz/27) * time.Microsecond
			}
		}

		if pcr >= 0 {
			now := time.Now()
			if lastPCR < 0 || pcr-lastPCR > 500*time.Millisecond || pcr-lastPCR < -500*time.Millisecond {
				startPCR = pcr
				startTime = now
			}
			if packetTime := startTime.Add(pcr - startPCR); now.Before(packetTime) {
				time.Sleep(packetTime.Sub(now))
			}
		}
		lastPCR = pcr

		if _, err := w.Write(buf); err != nil {
			return fmt.Errorf("write error: %w", err)
		}
	}

	return nil
}
