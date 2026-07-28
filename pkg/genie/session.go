package genie

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/kcaldas/genie/pkg/events"
	"github.com/kcaldas/genie/pkg/session"
)

// DefaultPersona is a simple implementation of the Persona interface
type DefaultPersona struct {
	ID     string
	Name   string
	Source string
}

func (p *DefaultPersona) GetID() string     { return p.ID }
func (p *DefaultPersona) GetName() string   { return p.Name }
func (p *DefaultPersona) GetSource() string { return p.Source }

// InMemorySession implements Session with event bus publishing
type InMemorySession struct {
	id                string
	genieHomeDir      string   // Where .genie/ config lives
	workingDir        string   // CWD for file operations
	allowedDirs       []string // Extra directories tools may access
	deniedPaths       []string // Glob patterns the agent must not touch
	readOnlyPaths     []string // Glob patterns the agent may read but not mutate
	commitAuthorName  string   // Opaque commit author name set by the host
	commitAuthorEmail string   // Opaque commit author email set by the host
	persona           Persona
	publisher         events.Publisher
	recorder          *session.Recorder
	createdAt         string
}

// NewSession creates a new session with genie home directory, working directory, allowed dirs, persona, publisher for broadcasting, and optional session recorder
func NewSession(genieHomeDir string, workingDir string, allowedDirs []string, persona Persona, publisher events.Publisher, recorder *session.Recorder) Session {
	return &InMemorySession{
		id:           newSessionID(),
		genieHomeDir: genieHomeDir,
		workingDir:   workingDir,
		allowedDirs:  allowedDirs,
		persona:      persona,
		publisher:    publisher,
		recorder:     recorder,
		createdAt:    time.Now().Format(time.RFC3339),
	}
}

// newSessionID returns a unique identifier for a session.
func newSessionID() string {
	buf := make([]byte, 8)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand never fails on supported platforms; fall back to a
		// timestamp so session creation cannot break.
		return fmt.Sprintf("session-%d", time.Now().UnixNano())
	}
	return "session-" + hex.EncodeToString(buf)
}

// GetWorkingDirectory returns the session's working directory (CWD for file operations)
func (s *InMemorySession) GetWorkingDirectory() string {
	return s.workingDir
}

// GetAllowedDirectories returns the extra directories that tools may access
func (s *InMemorySession) GetAllowedDirectories() []string {
	return s.allowedDirs
}

// GetGenieHomeDirectory returns the directory where .genie/ config lives
func (s *InMemorySession) GetGenieHomeDirectory() string {
	return s.genieHomeDir
}

// GetPersona returns the session's selected persona
func (s *InMemorySession) GetPersona() Persona {
	return s.persona
}

// SetPersona sets the session's selected persona and records actual changes
func (s *InMemorySession) SetPersona(persona Persona) {
	from := personaID(s.persona)
	to := personaID(persona)
	if from != to {
		s.recorder.AppendPersonaChange(from, to)
	}
	s.persona = persona
}

func personaID(persona Persona) string {
	if persona == nil {
		return ""
	}
	return persona.GetID()
}

// GetID returns the session's unique identifier
func (s *InMemorySession) GetID() string {
	return s.id
}

// GetCreatedAt returns the session's creation timestamp
func (s *InMemorySession) GetCreatedAt() string {
	return s.createdAt
}

// GetDeniedPaths returns the glob patterns the agent must not touch.
func (s *InMemorySession) GetDeniedPaths() []string {
	return s.deniedPaths
}

// GetReadOnlyPaths returns the glob patterns the agent may read but
// not mutate.
func (s *InMemorySession) GetReadOnlyPaths() []string {
	return s.readOnlyPaths
}

// GetCommitAuthor returns the opaque author identity gitCommit will
// attribute commits to. Empty values trigger the platform default.
func (s *InMemorySession) GetCommitAuthor() (string, string) {
	return s.commitAuthorName, s.commitAuthorEmail
}

// SetDeniedPaths overwrites the session's denied-path policy.
func (s *InMemorySession) SetDeniedPaths(patterns []string) {
	s.deniedPaths = patterns
}

// SetReadOnlyPaths overwrites the session's read-only policy.
func (s *InMemorySession) SetReadOnlyPaths(patterns []string) {
	s.readOnlyPaths = patterns
}

// SetCommitAuthor overwrites the opaque commit author identity.
func (s *InMemorySession) SetCommitAuthor(name, email string) {
	s.commitAuthorName = name
	s.commitAuthorEmail = email
}
