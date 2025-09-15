package logging

import (
    "io"
    "log"
    "os"
    "path/filepath"
    "sync"
)

type RotatingWriter struct {
    path   string
    maxB   int64
    file   *os.File
    mu     sync.Mutex
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
        _ = w.file.Close()
        _ = os.Rename(w.path, w.path+".1")
        if err := w.open(); err != nil {
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

