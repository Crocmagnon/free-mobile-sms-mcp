package sms_test

import (
	"context"
	"errors"
	"testing"

	"go.augendre.info/free-mobile-sms-mcp/domain/sms"
)

var errBoom = errors.New("boom")

type fakeSender struct {
	got []string
	err error
}

func (f *fakeSender) Send(_ context.Context, msg string) error {
	f.got = append(f.got, msg)

	return f.err
}

func TestSend_ForwardsTheMessageUntouched(t *testing.T) {
	t.Parallel()

	sender := &fakeSender{}
	msg := "  Build done ✅ & tests: 42/42\nnext = deploy  "

	if err := sms.NewService(sender).Send(t.Context(), msg); err != nil {
		t.Fatalf("Send: %v", err)
	}

	if len(sender.got) != 1 || sender.got[0] != msg {
		t.Fatalf("sender got %q, want exactly [%q]", sender.got, msg)
	}
}

func TestSend_RejectsAnEmptyMessageWithoutSending(t *testing.T) {
	t.Parallel()

	for _, msg := range []string{"", "   ", "\n\t "} {
		sender := &fakeSender{}

		err := sms.NewService(sender).Send(t.Context(), msg)
		if !errors.Is(err, sms.ErrEmptyMessage) {
			t.Errorf("Send(%q) = %v, want ErrEmptyMessage", msg, err)
		}

		if len(sender.got) != 0 {
			t.Errorf("Send(%q) reached the sender", msg)
		}
	}
}

func TestSend_ReturnsTheSenderError(t *testing.T) {
	t.Parallel()

	err := sms.NewService(&fakeSender{err: errBoom}).Send(t.Context(), "hi")
	if !errors.Is(err, errBoom) {
		t.Fatalf("Send = %v, want it to wrap %v", err, errBoom)
	}
}
