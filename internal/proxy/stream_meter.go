package proxy

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type streamMeterConfig struct {
	idleSec   int
	maxStream time.Duration
}

type meteringReader struct {
	src        io.Reader
	w          io.Writer
	flusher    http.Flusher
	cfg        streamMeterConfig
	ctx        context.Context
	usage      map[string]interface{}
	body       bytes.Buffer
	dataChunks int
	estOutput  int64
	holdMicro  int64
	softCapExceeded bool
}

func newMeteringReader(ctx context.Context, src io.Reader, w io.Writer, flusher http.Flusher, idleSec int, maxStream time.Duration, holdMicro int64) *meteringReader {
	return &meteringReader{
		src:       src,
		w:         w,
		flusher:   flusher,
		cfg:       streamMeterConfig{idleSec: idleSec, maxStream: maxStream},
		ctx:       ctx,
		holdMicro: holdMicro,
	}
}

func (m *meteringReader) copy() (int64, error) {
	br := bufio.NewReader(m.src)
	var total int64
	lastActivity := time.Now()
	idle := time.Duration(m.cfg.idleSec) * time.Second
	if idle <= 0 {
		idle = 30 * time.Second
	}
	deadline := time.Time{}
	if m.cfg.maxStream > 0 {
		deadline = time.Now().Add(m.cfg.maxStream)
	}

	for {
		if m.ctx != nil {
			select {
			case <-m.ctx.Done():
				m.finalizeUsage()
				return total, m.ctx.Err()
			default:
			}
		}
		if !deadline.IsZero() && time.Now().After(deadline) {
			m.finalizeUsage()
			return total, context.DeadlineExceeded
		}

		line, err := br.ReadBytes('\n')
		if len(line) > 0 {
			total += int64(len(line))
			m.body.Write(line)
			m.w.Write(line)
			if m.flusher != nil {
				m.flusher.Flush()
			}
			m.parseLine(line)
			lastActivity = time.Now()
		}
		if m.softCapExceeded {
			m.finalizeUsage()
			return total, fmt.Errorf("insufficient_credits")
		}
		if err != nil {
			if err == io.EOF {
				m.finalizeUsage()
				return total, nil
			}
			m.finalizeUsage()
			return total, err
		}
		if time.Since(lastActivity) > idle {
			m.w.Write([]byte(": keepalive\n\n"))
			if m.flusher != nil {
				m.flusher.Flush()
			}
			lastActivity = time.Now()
		}
	}
}

func (m *meteringReader) parseLine(line []byte) {
	s := strings.TrimSpace(string(line))
	if !strings.HasPrefix(s, "data: ") {
		return
	}
	data := strings.TrimPrefix(s, "data: ")
	if data == "[DONE]" {
		return
	}
	m.dataChunks++
	m.estOutput += int64(len(data)) / 4
	if m.holdMicro > 0 && m.estOutput*900 > m.holdMicro {
		m.softCapExceeded = true
	}

	var chunk map[string]interface{}
	if err := json.Unmarshal([]byte(data), &chunk); err != nil {
		return
	}
	if usage, ok := chunk["usage"].(map[string]interface{}); ok {
		m.usage = usage
	}
}

func (m *meteringReader) finalizeUsage() {
	if m.usage != nil {
		return
	}
	if m.dataChunks > 0 {
		m.usage = map[string]interface{}{
			"output_tokens": m.estOutput,
			"input_tokens":  0,
			"fallback":      "chunk_estimate",
		}
	}
}

func parseJSONUsage(data []byte) map[string]interface{} {
	var v map[string]interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil
	}
	if usage, ok := v["usage"].(map[string]interface{}); ok {
		return usage
	}
	return v
}
