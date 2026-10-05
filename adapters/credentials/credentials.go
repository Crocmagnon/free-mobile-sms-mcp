// Package credentials stores the Free Mobile login and API key in a file only the user's own account can read.
package credentials

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// PathEnv names the environment variable that overrides the credentials file location.
// It carries a path only, never the key.
const PathEnv = "FREE_MOBILE_SMS_MCP_CREDENTIALS"

const dirMode = 0o700

var (
	// ErrNoCredentials means no credentials file exists yet.
	ErrNoCredentials = errors.New(
		"no Free Mobile credentials stored: run `free-mobile-sms-mcp login` in your own terminal",
	)
	// ErrIncomplete means the user or the key is empty.
	ErrIncomplete = errors.New("the credentials file needs both a user and a pass")
	// ErrNoPath means the store has no file location.
	ErrNoPath = errors.New(
		"no user configuration directory on this platform: set " + PathEnv + " to the file the credentials belong in",
	)
)

// Credentials is the Free Mobile login and API key.
type Credentials struct {
	User string `json:"user"`
	Pass string `json:"pass"`
}

// Store reads and writes the credentials file.
type Store struct{ path string }

// New returns the store at $FREE_MOBILE_SMS_MCP_CREDENTIALS, or in the user configuration directory.
// Without either, the first call on the store returns ErrNoPath.
func New() Store {
	if p := os.Getenv(PathEnv); p != "" {
		return Store{path: p}
	}

	dir, err := os.UserConfigDir()
	if err != nil {
		return Store{}
	}

	return Store{path: filepath.Join(dir, "free-mobile-sms-mcp", "credentials.json")}
}

// NewAt returns the store at path.
func NewAt(path string) Store { return Store{path: path} }

// Path returns the credentials file location.
func (s Store) Path() string { return s.path }

// Get reads the credentials. Errors name the path, never the file contents.
func (s Store) Get() (Credentials, error) {
	if s.path == "" {
		return Credentials{}, ErrNoPath
	}

	data, err := os.ReadFile(s.path)
	if errors.Is(err, os.ErrNotExist) {
		return Credentials{}, ErrNoCredentials
	}

	if err != nil {
		return Credentials{}, fmt.Errorf("read %s: %w", s.path, err)
	}

	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return Credentials{}, fmt.Errorf("decode %s: %w", s.path, scrub(err))
	}

	if creds.User == "" || creds.Pass == "" {
		return Credentials{}, fmt.Errorf("%s: %w", s.path, ErrIncomplete)
	}

	return creds, nil
}

// Save writes the credentials atomically, readable by the owner only.
func (s Store) Save(creds Credentials) error {
	if s.path == "" {
		return ErrNoPath
	}

	if creds.User == "" || creds.Pass == "" {
		return ErrIncomplete
	}

	//nolint:gosec // the key is written to the owner-only file on purpose
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("encode credentials: %w", err)
	}

	dir := filepath.Dir(s.path)
	if err := os.MkdirAll(dir, dirMode); err != nil {
		return fmt.Errorf("create %s: %w", dir, err)
	}

	tmp, err := os.CreateTemp(dir, ".credentials-*.json")
	if err != nil {
		return fmt.Errorf("create a temporary file in %s: %w", dir, err)
	}

	_, werr := tmp.Write(append(data, '\n'))

	cerr := tmp.Close()
	if err := errors.Join(werr, cerr); err != nil {
		_ = os.Remove(tmp.Name())

		return fmt.Errorf("write %s: %w", tmp.Name(), err)
	}

	if err := os.Rename(tmp.Name(), s.path); err != nil {
		_ = os.Remove(tmp.Name())

		return fmt.Errorf("replace %s: %w", s.path, err)
	}

	return nil
}

// Delete removes the credentials file; an absent file is not an error.
func (s Store) Delete() error {
	if s.path == "" {
		return ErrNoPath
	}

	if err := os.Remove(s.path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("remove %s: %w", s.path, err)
	}

	return nil
}

// scrub keeps decode errors from echoing file content.
func scrub(err error) error {
	if syn, ok := errors.AsType[*json.SyntaxError](err); ok {
		return fmt.Errorf("invalid JSON at byte %d", syn.Offset) //nolint:err113 // message only
	}

	if typ, ok := errors.AsType[*json.UnmarshalTypeError](err); ok {
		return fmt.Errorf("field %q has the wrong type", typ.Field) //nolint:err113 // message only
	}

	return errors.New("invalid JSON") //nolint:err113 // message only
}
