package session

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DiskJSONL is a Storage that appends directly to a local JSONL file.
// Suitable for real filesystems with cheap appends; for FUSE-style mounts
// without real append use SpoolCheckpoint instead.
type DiskJSONL struct {
	mu        sync.Mutex
	file      *os.File
	hasHeader bool
}

// NewDiskJSONL opens (or creates) the session file at path for appending.
// Parent directories are created. When the file already has content the
// header write is skipped on this handle (continuation).
func NewDiskJSONL(path string) (*DiskJSONL, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create session dir: %w", err)
	}
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open session file: %w", err)
	}
	info, err := file.Stat()
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("stat session file: %w", err)
	}
	return &DiskJSONL{file: file, hasHeader: info.Size() > 0}, nil
}

// WriteHeader writes the header line unless the file already has content.
func (d *DiskJSONL) WriteHeader(header []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.hasHeader {
		return nil
	}
	if err := writeLine(d.file, header); err != nil {
		return err
	}
	d.hasHeader = true
	return nil
}

// AppendEntry appends one entry line.
func (d *DiskJSONL) AppendEntry(entry []byte) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := writeLine(d.file, entry); err != nil {
		return err
	}
	d.hasHeader = true
	return nil
}

// Checkpoint fsyncs the file.
func (d *DiskJSONL) Checkpoint() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.file.Sync()
}

// Close fsyncs and closes the file.
func (d *DiskJSONL) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	syncErr := d.file.Sync()
	closeErr := d.file.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

func writeLine(file *os.File, line []byte) error {
	if _, err := file.Write(append(append([]byte(nil), line...), '\n')); err != nil {
		return fmt.Errorf("append session line: %w", err)
	}
	return nil
}
