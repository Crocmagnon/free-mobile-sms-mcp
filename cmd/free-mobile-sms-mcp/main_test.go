package main

import (
	"errors"
	"strings"
	"testing"

	"go.augendre.info/free-mobile-sms-mcp/adapters/credentials"
)

var errSecretNotWanted = errors.New("the secret must not be read")

func TestReadCredentials_ReadsLoginThenSecret(t *testing.T) {
	t.Parallel()

	var out strings.Builder

	got, err := readCredentials(strings.NewReader("12345678\n"), &out, func() ([]byte, error) {
		return []byte("k3y"), nil
	})
	if err != nil {
		t.Fatalf("readCredentials: %v", err)
	}

	if want := (credentials.Credentials{User: "12345678", Pass: "k3y"}); got != want {
		t.Errorf("got %+v, want %+v", got, want)
	}

	if !strings.Contains(out.String(), "Free Mobile login: ") {
		t.Errorf("output %q lacks the login prompt", out.String())
	}

	if strings.Contains(out.String(), "k3y") {
		t.Errorf("output %q echoes the secret", out.String())
	}
}

func TestReadCredentials_TrimsTheLogin(t *testing.T) {
	t.Parallel()

	got, err := readCredentials(
		strings.NewReader("  12345678 \r\n"),
		&strings.Builder{},
		func() ([]byte, error) {
			return []byte(" k3y \n"), nil
		},
	)
	if err != nil {
		t.Fatalf("readCredentials: %v", err)
	}

	if got.User != "12345678" || got.Pass != "k3y" {
		t.Errorf("got %+v, want trimmed values", got)
	}
}

func TestReadCredentials_RefusesEmpty(t *testing.T) {
	t.Parallel()

	_, err := readCredentials(strings.NewReader("\n"), &strings.Builder{}, func() ([]byte, error) {
		return nil, errSecretNotWanted
	})
	if !errors.Is(err, errNothingEntered) {
		t.Errorf("empty login: got %v, want errNothingEntered", err)
	}

	_, err = readCredentials(
		strings.NewReader("12345678\n"),
		&strings.Builder{},
		func() ([]byte, error) {
			return []byte("  "), nil
		},
	)
	if !errors.Is(err, errNothingEntered) {
		t.Errorf("empty secret: got %v, want errNothingEntered", err)
	}
}

func TestVersion_IsNeverEmpty(t *testing.T) {
	t.Parallel()

	if version() == "" {
		t.Error("version() is empty")
	}
}
