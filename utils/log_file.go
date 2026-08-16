package utils

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

type RotateWriter struct {
	mu         sync.Mutex
	filename   string
	maxSize    int64
	maxBackups int

	file *os.File
	size int64
}

func NewRotateWriter(
	filename string,
	maxSize int64,
	maxBackups int,
) (*RotateWriter, error) {

	f, err := os.OpenFile(
		filename,
		os.O_CREATE|os.O_APPEND|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return nil, err
	}

	stat, err := f.Stat()
	if err != nil {
		f.Close()
		return nil, err
	}

	return &RotateWriter{
		filename:   filename,
		maxSize:    maxSize,
		maxBackups: maxBackups,
		file:       f,
		size:       stat.Size(),
	}, nil
}

func (w *RotateWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.size+int64(len(p)) > w.maxSize {
		if err := w.rotate(); err != nil {
			return 0, err
		}
	}

	n, err := w.file.Write(p)
	w.size += int64(n)

	return n, err
}

func (w *RotateWriter) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if w.file != nil {
		return w.file.Close()
	}
	return nil
}

func (w *RotateWriter) rotate() error {
	if w.file != nil {
		if err := w.file.Close(); err != nil {
			return err
		}
	}

	oldest := fmt.Sprintf("%s.%d", w.filename, w.maxBackups)
	_ = os.Remove(oldest)

	for i := w.maxBackups - 1; i >= 1; i-- {
		src := fmt.Sprintf("%s.%d", w.filename, i)
		dst := fmt.Sprintf("%s.%d", w.filename, i+1)

		if _, err := os.Stat(src); err == nil {
			if err := os.Rename(src, dst); err != nil {
				return err
			}
		}
	}

	if _, err := os.Stat(w.filename); err == nil {
		if err := os.Rename(
			w.filename,
			fmt.Sprintf("%s.1", w.filename),
		); err != nil {
			return err
		}
	}

	f, err := os.OpenFile(
		w.filename,
		os.O_CREATE|os.O_TRUNC|os.O_WRONLY,
		0644,
	)
	if err != nil {
		return err
	}

	w.file = f
	w.size = 0

	return nil
}

func (w *RotateWriter) CurrentFile() string {
	return filepath.Clean(w.filename)
}

var _ io.Writer = (*RotateWriter)(nil)
