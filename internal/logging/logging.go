package logging

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

const maxArchives = 5

type RotatingWriter struct {
	path string
	maxB int64
	file *os.File
	mu   sync.Mutex
}

func NewRotatingWriter(path string, maxBytes int64) (*RotatingWriter, error) {
	rw := &RotatingWriter{path: path, maxB: maxBytes}
	if err := rw.open(); err != nil {
		return nil, err
	}
	return rw, nil
}

func (w *RotatingWriter) open() error {
	if err := os.MkdirAll(filepath.Dir(w.path), 0o755); err != nil {
		return err
	}
	f, err := os.OpenFile(w.path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	w.file = f
	return nil
}

func (w *RotatingWriter) rotate() error {
	if w.file != nil {
		_ = w.file.Close()
		w.file = nil
	}
	for i := maxArchives; i >= 1; i-- {
		src := w.path
		if i > 1 {
			src = w.path + "." + fmt.Sprint(i-1)
		}
		dst := w.path + "." + fmt.Sprint(i)
		if _, err := os.Stat(src); err == nil {
			_ = os.Remove(dst)
			_ = os.Rename(src, dst)
		}
	}
	return w.open()
}

func (w *RotatingWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file == nil {
		if err := w.open(); err != nil {
			return 0, err
		}
	}
	fi, _ := w.file.Stat()
	if fi != nil && fi.Size()+int64(len(p)) > w.maxB {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}
	return w.file.Write(p)
}

func (w *RotatingWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

// Level defines logging severity.
type Level int

const (
	Trace Level = iota
	Debug
	Info
	Warn
	Error
)

func parseLevel(s string) Level {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "trace":
		return Trace
	case "debug":
		return Debug
	case "warn":
		return Warn
	case "error":
		return Error
	default:
		return Info
	}
}

// Logger is a minimal structured logger.
type Logger interface {
	With(fields map[string]any) Logger
	Log(level Level, msg string, fields map[string]any)
	Trace(msg string, fields map[string]any)
	Debug(msg string, fields map[string]any)
	Info(msg string, fields map[string]any)
	Warn(msg string, fields map[string]any)
	Error(msg string, fields map[string]any)
	Writer() io.Writer // for redirecting std log
}

type jsonLogger struct {
	w      io.Writer
	level  Level
	fields map[string]any
	asText bool
}

// New creates a structured logger writing to w using given format ("json" or "text") and level string.
func New(w io.Writer, format, level string) Logger {
	jl := &jsonLogger{w: w, level: parseLevel(level), fields: map[string]any{}}
	if strings.ToLower(strings.TrimSpace(format)) == "text" {
		jl.asText = true
	}
	return jl
}

func (l *jsonLogger) With(fields map[string]any) Logger {
	if len(fields) == 0 {
		return l
	}
	nf := make(map[string]any, len(l.fields)+len(fields))
	for k, v := range l.fields {
		nf[k] = v
	}
	for k, v := range fields {
		nf[k] = v
	}
	return &jsonLogger{w: l.w, level: l.level, fields: nf, asText: l.asText}
}

func (l *jsonLogger) Writer() io.Writer { return &stdAdapter{l: l} }

func (l *jsonLogger) enabled(level Level) bool { return level >= l.level }

func (l *jsonLogger) Log(level Level, msg string, fields map[string]any) {
	if !l.enabled(level) {
		return
	}
	ev := make(map[string]any, len(l.fields)+len(fields)+3)
	for k, v := range l.fields {
		ev[k] = v
	}
	for k, v := range fields {
		ev[k] = v
	}
	ev["ts"] = time.Now().Format(time.RFC3339Nano)
	switch level {
	case Trace:
		ev["lvl"] = "trace"
	case Debug:
		ev["lvl"] = "debug"
	case Warn:
		ev["lvl"] = "warn"
	case Error:
		ev["lvl"] = "error"
	default:
		ev["lvl"] = "info"
	}
	ev["msg"] = msg
	if l.asText {
		// Simple text: key=value pairs
		var sb strings.Builder
		sb.WriteString(ev["ts"].(string))
		sb.WriteString(" ")
		sb.WriteString(ev["lvl"].(string))
		sb.WriteString(" ")
		sb.WriteString(msg)
		for k, v := range ev {
			if k == "ts" || k == "lvl" || k == "msg" {
				continue
			}
			sb.WriteString(" ")
			sb.WriteString(k)
			sb.WriteString("=")
			sb.WriteString(fmt.Sprint(v))
		}
		sb.WriteString("\n")
		_, _ = l.w.Write([]byte(sb.String()))
		return
	}
	b, _ := json.Marshal(ev)
	b = append(b, '\n')
	_, _ = l.w.Write(b)
}

func (l *jsonLogger) Trace(msg string, fields map[string]any) { l.Log(Trace, msg, fields) }
func (l *jsonLogger) Debug(msg string, fields map[string]any) { l.Log(Debug, msg, fields) }
func (l *jsonLogger) Info(msg string, fields map[string]any)  { l.Log(Info, msg, fields) }
func (l *jsonLogger) Warn(msg string, fields map[string]any)  { l.Log(Warn, msg, fields) }
func (l *jsonLogger) Error(msg string, fields map[string]any) { l.Log(Error, msg, fields) }

// stdAdapter adapts legacy log.Printf to structured logs by parsing key=value tokens when present.
type stdAdapter struct{ l *jsonLogger }

func (a *stdAdapter) Write(p []byte) (int, error) {
	// strip trailing newline
	s := strings.TrimSpace(string(p))
	if s == "" {
		return len(p), nil
	}
	msg, fields := splitMsgFields(s)
	a.l.Info(msg, fields)
	return len(p), nil
}

func splitMsgFields(s string) (string, map[string]any) {
	// naive parse: tokens separated by space; key=value pairs become fields
	parts := strings.Fields(s)
	fields := map[string]any{}
	var msgParts []string
	for _, p := range parts {
		if i := strings.IndexByte(p, '='); i > 0 {
			k := p[:i]
			v := p[i+1:]
			fields[k] = v
			continue
		}
		msgParts = append(msgParts, p)
	}
	return strings.Join(msgParts, " "), fields
}

// Setup sets up a rotating file writer and returns a structured logger and the writer closer.
func Setup(path, format, level string) (Logger, io.Closer, error) {
	rw, err := NewRotatingWriter(path, 5*1024*1024)
	if err != nil {
		return nil, nil, err
	}
	mw := io.MultiWriter(os.Stdout, rw)
	lg := New(mw, format, level)
	return lg, rw, nil
}
