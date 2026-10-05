package freemobile_test

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"

	"go.augendre.info/free-mobile-sms-mcp/adapters/freemobile"
)

const (
	testUser = "12345678"
	testPass = "s3cr3t-k3y"
)

func TestSend_PostsTheFormFields(t *testing.T) {
	t.Parallel()

	msg := "Déploiement OK ✅ & a=b+c\nligne 2"

	var gotMethod, gotUser, gotPass, gotMsg, gotQuery string

	server := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, req *http.Request) {
		gotMethod, gotQuery = req.Method, req.URL.RawQuery
		if err := req.ParseForm(); err != nil {
			t.Errorf("ParseForm: %v", err)
		}

		gotUser, gotPass, gotMsg = req.PostForm.Get(
			"user",
		), req.PostForm.Get(
			"pass",
		), req.PostForm.Get(
			"msg",
		)
	}))
	t.Cleanup(server.Close)

	err := freemobile.New(server.Client(), server.URL, testUser, testPass).Send(t.Context(), msg)
	if err != nil {
		t.Fatalf("Send: %v", err)
	}

	if gotMethod != http.MethodPost {
		t.Errorf("method = %s, want POST", gotMethod)
	}

	if gotQuery != "" {
		t.Errorf("query = %q, want nothing in the URL", gotQuery)
	}

	if gotUser != testUser || gotPass != testPass || gotMsg != msg {
		t.Errorf("form = user %q pass %q msg %q", gotUser, gotPass, gotMsg)
	}
}

func TestSend_MapsTheStatusCodes(t *testing.T) {
	t.Parallel()

	for code, want := range map[int]error{
		http.StatusBadRequest:          freemobile.ErrBadRequest,
		http.StatusPaymentRequired:     freemobile.ErrRateLimited,
		http.StatusForbidden:           freemobile.ErrForbidden,
		http.StatusInternalServerError: freemobile.ErrServer,
		http.StatusNotFound:            freemobile.ErrUnexpectedStatus,
		http.StatusBadGateway:          freemobile.ErrUnexpectedStatus,
		http.StatusCreated:             freemobile.ErrUnexpectedStatus,
	} {
		server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(code)
		}))

		err := freemobile.New(server.Client(), server.URL, testUser, testPass).
			Send(t.Context(), "hi")

		server.Close()

		if !errors.Is(err, want) {
			t.Errorf("HTTP %d: Send = %v, want %v", code, err, want)
		}

		if err != nil && strings.Contains(err.Error(), testPass) {
			t.Errorf("HTTP %d: the error leaks the key: %v", code, err)
		}

		if errors.Is(want, freemobile.ErrUnexpectedStatus) && err != nil &&
			!strings.Contains(err.Error(), strconv.Itoa(code)) {
			t.Errorf("HTTP %d: the error does not name the code: %v", code, err)
		}
	}
}

func TestSend_UnreachableServerDoesNotLeakTheKey(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.NotFoundHandler())
	url := server.URL

	server.Close()

	err := freemobile.New(http.DefaultClient, url, testUser, testPass).Send(t.Context(), "hi")
	if err == nil {
		t.Fatal("Send succeeded against a closed server")
	}

	if strings.Contains(err.Error(), testPass) {
		t.Errorf("the error leaks the key: %v", err)
	}
}

func TestSend_RedirectIsNotASuccess(t *testing.T) {
	t.Parallel()

	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	defer target.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, target.URL, http.StatusFound)
	}))
	defer server.Close()

	err := freemobile.New(freemobile.NewHTTPClient(), server.URL, testUser, testPass).
		Send(t.Context(), "hi")
	if !errors.Is(err, freemobile.ErrUnexpectedStatus) {
		t.Errorf("Send = %v, want %v", err, freemobile.ErrUnexpectedStatus)
	}
}
