package credentials_test

import (
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"go.augendre.info/free-mobile-sms-mcp/adapters/credentials"
)

func newStore(t *testing.T) (credentials.Store, string) {
	t.Helper()

	path := filepath.Join(t.TempDir(), "sub", "credentials.json")

	return credentials.NewAt(path), path
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatalf("MkdirAll: %v", err)
	}

	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
}

func TestSaveThenGet_RoundTrips(t *testing.T) {
	t.Parallel()

	store, _ := newStore(t)
	want := credentials.Credentials{User: "12345678", Pass: "k3y"}

	if err := store.Save(want); err != nil {
		t.Fatalf("Save: %v", err)
	}

	got, err := store.Get()
	if err != nil {
		t.Fatalf("Get: %v", err)
	}

	if got != want {
		t.Errorf("Get = %+v, want %+v", got, want)
	}
}

func TestSave_KeepsTheFileOwnerOnly(t *testing.T) {
	t.Parallel()

	if runtime.GOOS == "windows" {
		t.Skip("no unix modes")
	}

	store, path := newStore(t)

	if err := store.Save(credentials.Credentials{User: "u", Pass: "p"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	info, err := os.Stat(path)
	if err != nil {
		t.Fatalf("Stat: %v", err)
	}

	if info.Mode().Perm() != 0o600 {
		t.Errorf("mode = %v, want 0600", info.Mode().Perm())
	}
}

func TestGet_AbsentFile(t *testing.T) {
	t.Parallel()

	store, _ := newStore(t)

	if _, err := store.Get(); !errors.Is(err, credentials.ErrNoCredentials) {
		t.Errorf("Get err = %v, want ErrNoCredentials", err)
	}
}

func TestGet_MalformedFile(t *testing.T) {
	t.Parallel()

	store, path := newStore(t)
	writeFile(t, path, "{not json")

	_, err := store.Get()
	if err == nil {
		t.Fatal("Get err = nil, want a decode error")
	}

	if errors.Is(err, credentials.ErrNoCredentials) {
		t.Errorf("Get err = %v, must not be ErrNoCredentials", err)
	}

	if !strings.Contains(err.Error(), path) {
		t.Errorf("Get err = %q, want it to name %q", err, path)
	}
}

func TestGet_IncompleteFile(t *testing.T) {
	t.Parallel()

	store, path := newStore(t)
	writeFile(t, path, `{"user":"12345678"}`)

	if _, err := store.Get(); !errors.Is(err, credentials.ErrIncomplete) {
		t.Errorf("Get err = %v, want ErrIncomplete", err)
	}
}

func TestSave_RefusesIncomplete(t *testing.T) {
	t.Parallel()

	store, path := newStore(t)

	if err := store.Save(
		credentials.Credentials{User: "x"},
	); !errors.Is(
		err,
		credentials.ErrIncomplete,
	) {
		t.Errorf("Save err = %v, want ErrIncomplete", err)
	}

	if _, err := os.Stat(path); !errors.Is(err, os.ErrNotExist) {
		t.Errorf("Stat err = %v, want the file to be absent", err)
	}
}

func TestDelete_RemovesAndToleratesAbsence(t *testing.T) {
	t.Parallel()

	store, _ := newStore(t)

	if err := store.Save(credentials.Credentials{User: "u", Pass: "p"}); err != nil {
		t.Fatalf("Save: %v", err)
	}

	if err := store.Delete(); err != nil {
		t.Fatalf("Delete: %v", err)
	}

	if _, err := store.Get(); !errors.Is(err, credentials.ErrNoCredentials) {
		t.Errorf("Get err = %v, want ErrNoCredentials", err)
	}

	if err := store.Delete(); err != nil {
		t.Errorf("second Delete: %v", err)
	}
}

func TestNew_HonoursTheEnv(t *testing.T) {
	p := filepath.Join(t.TempDir(), "elsewhere.json")
	t.Setenv(credentials.PathEnv, p)

	if got := credentials.New().Path(); got != p {
		t.Errorf("Path = %q, want %q", got, p)
	}
}

func TestEmptyPath_ReportsErrNoPath(t *testing.T) {
	t.Parallel()

	store := credentials.NewAt("")

	if _, err := store.Get(); !errors.Is(err, credentials.ErrNoPath) {
		t.Errorf("Get err = %v, want ErrNoPath", err)
	}

	if err := store.Save(
		credentials.Credentials{User: "u", Pass: "p"},
	); !errors.Is(
		err,
		credentials.ErrNoPath,
	) {
		t.Errorf("Save err = %v, want ErrNoPath", err)
	}

	if err := store.Delete(); !errors.Is(err, credentials.ErrNoPath) {
		t.Errorf("Delete err = %v, want ErrNoPath", err)
	}
}

func TestGet_DecodeErrorsDoNotEchoTheFile(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, content, leak, mention string
	}{
		{"truncated JSON", `{"user":"u","pass":"SECRETKEY`, "SECRETKEY", ""},
		{"wrong type", `{"user":"u","pass":42}`, "42", "pass"},
		{"stray character", `{"user":"u","pass":Q}`, "Q", ""},
	}

	for _, tcase := range tests {
		t.Run(tcase.name, func(t *testing.T) {
			t.Parallel()

			store, path := newStore(t)
			writeFile(t, path, tcase.content)

			_, err := store.Get()
			if err == nil {
				t.Fatal("Get succeeded on a malformed file")
			}

			msg := strings.ReplaceAll(err.Error(), path, "")
			if strings.Contains(msg, tcase.leak) {
				t.Errorf("the error echoes the file content: %v", err)
			}

			if !strings.Contains(msg, tcase.mention) {
				t.Errorf("the error does not name %q: %v", tcase.mention, err)
			}
		})
	}
}
