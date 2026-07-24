package session

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// SpoolCheckpoint is a Storage for mounts without real append (gcsfuse):
// entries append to a local spool file, and Checkpoint publishes the whole
// spool to the target with an atomic tmp+rename — exactly one whole-object
// upload per checkpoint on FUSE.
//
// Construction seeds the spool from an existing target so a restarted pod
// continues the session where the last checkpoint left it.
type SpoolCheckpoint struct {
	mu         sync.Mutex
	spool      *os.File
	spoolPath  string
	targetPath string
	hasHeader  bool
}

// NewSpoolCheckpoint opens a spool file under spoolDir for the session
// targeted at targetPath. When the spool is empty and the target already
// exists, the spool is seeded from the target (pod-restart continuation).
func NewSpoolCheckpoint(spoolDir, targetPath string) (*SpoolCheckpoint, error) {
	if err := os.MkdirAll(spoolDir, 0o700); err != nil {
		return nil, fmt.Errorf("create spool dir: %w", err)
	}
	spoolPath := filepath.Join(spoolDir, spoolFileName(targetPath))

	if info, err := os.Stat(spoolPath); err != nil || info.Size() == 0 {
		if err := seedSpoolFromTarget(spoolPath, targetPath); err != nil {
			return nil, err
		}
	}

	spool, err := os.OpenFile(spoolPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, fmt.Errorf("open spool file: %w", err)
	}
	info, err := spool.Stat()
	if err != nil {
		spool.Close()
		return nil, fmt.Errorf("stat spool file: %w", err)
	}

	return &SpoolCheckpoint{
		spool:      spool,
		spoolPath:  spoolPath,
		targetPath: targetPath,
		hasHeader:  info.Size() > 0,
	}, nil
}

// WriteHeader writes the header line unless the spool already has content
// (freshly seeded from the target or reused by the same process).
func (s *SpoolCheckpoint) WriteHeader(header []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.hasHeader {
		return nil
	}
	if err := writeLine(s.spool, header); err != nil {
		return err
	}
	s.hasHeader = true
	return nil
}

// AppendEntry appends one entry line to the local spool. The target is not
// touched until the next Checkpoint.
func (s *SpoolCheckpoint) AppendEntry(entry []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := writeLine(s.spool, entry); err != nil {
		return err
	}
	s.hasHeader = true
	return nil
}

// Checkpoint fsyncs the spool and atomically publishes its full contents to
// the target: copy to a temp file next to the target, then rename over it.
func (s *SpoolCheckpoint) Checkpoint() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if err := s.spool.Sync(); err != nil {
		return fmt.Errorf("sync spool: %w", err)
	}

	targetDir := filepath.Dir(s.targetPath)
	if err := os.MkdirAll(targetDir, 0o700); err != nil {
		return fmt.Errorf("create target dir: %w", err)
	}

	tmpPath := filepath.Join(targetDir, ".tmp-"+filepath.Base(s.targetPath)+"-"+randomSuffix())
	if err := copyFile(s.spoolPath, tmpPath); err != nil {
		os.Remove(tmpPath)
		return err
	}
	if err := os.Rename(tmpPath, s.targetPath); err != nil {
		os.Remove(tmpPath)
		return fmt.Errorf("publish session checkpoint: %w", err)
	}
	return nil
}

// Close fsyncs and closes the spool. The Recorder checkpoints before Close,
// so the target already holds the final state.
func (s *SpoolCheckpoint) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	syncErr := s.spool.Sync()
	closeErr := s.spool.Close()
	if syncErr != nil {
		return syncErr
	}
	return closeErr
}

// spoolFileName derives a collision-safe spool name from the full target
// path (same basename in different conversations must not clash).
func spoolFileName(targetPath string) string {
	sum := sha256.Sum256([]byte(targetPath))
	return hex.EncodeToString(sum[:8]) + "-" + filepath.Base(targetPath)
}

func seedSpoolFromTarget(spoolPath, targetPath string) error {
	if _, err := os.Stat(targetPath); err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("stat session target: %w", err)
	}
	if err := copyFile(targetPath, spoolPath); err != nil {
		return fmt.Errorf("seed spool from target: %w", err)
	}
	return nil
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("open %s: %w", src, err)
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return fmt.Errorf("create %s: %w", dst, err)
	}
	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return fmt.Errorf("copy %s: %w", dst, err)
	}
	if err := out.Sync(); err != nil {
		out.Close()
		return fmt.Errorf("sync %s: %w", dst, err)
	}
	return out.Close()
}

func randomSuffix() string {
	buf := make([]byte, 4)
	if _, err := rand.Read(buf); err != nil {
		return "0"
	}
	return hex.EncodeToString(buf)
}
