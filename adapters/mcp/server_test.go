package mcp_test

import (
	"context"
	"errors"
	"slices"
	"strings"
	"testing"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"

	mcpadapter "go.augendre.info/free-mobile-sms-mcp/adapters/mcp"
)

var errRateLimited = errors.New("rate limited")

type fakeSMS struct {
	got []string
	err error
}

func (f *fakeSMS) Send(_ context.Context, msg string) error {
	f.got = append(f.got, msg)

	return f.err
}

func connect(t *testing.T, service mcpadapter.SMS) *sdk.ClientSession {
	t.Helper()

	clientTransport, serverTransport := sdk.NewInMemoryTransports()

	serverSession, err := mcpadapter.NewServer(service, "test").Serve(t.Context(), serverTransport)
	if err != nil {
		t.Fatalf("Serve: %v", err)
	}

	t.Cleanup(func() { _ = serverSession.Wait() })

	clientSession, err := sdk.NewClient(&sdk.Implementation{Name: "test"}, nil).
		Connect(t.Context(), clientTransport, nil)
	if err != nil {
		t.Fatalf("Connect: %v", err)
	}

	t.Cleanup(func() { _ = clientSession.Close() })

	return clientSession
}

func callSendSMS(t *testing.T, service mcpadapter.SMS, msg string) *sdk.CallToolResult {
	t.Helper()

	result, err := connect(t, service).CallTool(t.Context(), &sdk.CallToolParams{
		Name:      "send_sms",
		Arguments: map[string]any{"message": msg},
	})
	if err != nil {
		t.Fatalf("CallTool: %v", err)
	}

	return result
}

func resultText(t *testing.T, result *sdk.CallToolResult) string {
	t.Helper()

	if len(result.Content) != 1 {
		t.Fatalf("content = %v, want one item", result.Content)
	}

	text, ok := result.Content[0].(*sdk.TextContent)
	if !ok {
		t.Fatalf("content = %T, want *TextContent", result.Content[0])
	}

	return text.Text
}

func TestTools_OnlySendSMS(t *testing.T) {
	t.Parallel()

	list, err := connect(t, &fakeSMS{}).ListTools(t.Context(), nil)
	if err != nil {
		t.Fatalf("ListTools: %v", err)
	}

	if len(list.Tools) != 1 || list.Tools[0].Name != "send_sms" {
		t.Fatalf("tools = %v, want only send_sms", list.Tools)
	}

	schema, ok := list.Tools[0].InputSchema.(map[string]any)
	if !ok {
		t.Fatalf("input schema = %T, want an object", list.Tools[0].InputSchema)
	}

	props, _ := schema["properties"].(map[string]any)
	if _, ok := props["message"]; !ok {
		t.Errorf("properties = %v, want message", props)
	}

	required, _ := schema["required"].([]any)
	if !slices.Contains(required, any("message")) {
		t.Errorf("required = %v, want message", required)
	}
}

func TestSendSMS_Success(t *testing.T) {
	t.Parallel()

	fake := &fakeSMS{}

	result := callSendSMS(t, fake, "done ✅")

	if result.IsError {
		t.Errorf("IsError = true, want false")
	}

	if text := resultText(t, result); text != "SMS sent." {
		t.Errorf("text = %q, want %q", text, "SMS sent.")
	}

	if !slices.Equal(fake.got, []string{"done ✅"}) {
		t.Errorf("sent = %q", fake.got)
	}
}

func TestSendSMS_ErrorIsAToolError(t *testing.T) {
	t.Parallel()

	result := callSendSMS(t, &fakeSMS{err: errRateLimited}, "hi")

	if !result.IsError {
		t.Error("IsError = false, want true")
	}

	if text := resultText(t, result); !strings.Contains(text, "rate limited") {
		t.Errorf("text = %q, want it to contain the error", text)
	}
}
