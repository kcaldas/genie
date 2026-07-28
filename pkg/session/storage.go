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
	// Checkpoint is the turn-boundary signal. File storages make
	// everything appended so far durable (fsync); streaming storages may
	// treat it as a non-blocking flush hint.
	Checkpoint() error
	// Close releases resources. The Recorder checkpoints before closing.
	Close() error
}
