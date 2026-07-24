package session

// Storage is the byte sink a Recorder writes to. The recorder owns
// marshaling and caps; storages only move bytes.
//
// WriteHeader is idempotent per file: implementations skip it when the
// underlying file already has content (reopen/continuation).
type Storage interface {
	// WriteHeader writes the session header line. No-op when the
	// destination already holds data.
	WriteHeader(header []byte) error
	// AppendEntry appends one JSONL entry line.
	AppendEntry(entry []byte) error
	// Checkpoint makes everything appended so far durable at the
	// destination (fsync for direct files, whole-file publish for
	// spooled storages).
	Checkpoint() error
	// Close releases resources. The Recorder checkpoints before closing.
	Close() error
}
