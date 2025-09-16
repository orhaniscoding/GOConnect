package logging

import (
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
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

func SetupLogger(path string) (*log.Logger, io.Closer, error) {
	rw, err := NewRotatingWriter(path, 5*1024*1024)
	if err != nil {
		return nil, nil, err
	}
	mw := io.MultiWriter(os.Stdout, rw)
	lg := log.New(mw, "GOConnect ", log.LstdFlags|log.Lmicroseconds)
	return lg, rw, nil
}
