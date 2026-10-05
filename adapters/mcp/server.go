// Package mcp exposes the SMS use case as an MCP tool over stdio.
package mcp

import (
	"context"
	"fmt"

	sdk "github.com/modelcontextprotocol/go-sdk/mcp"
)

const sendSMSDescription = "Send an SMS to the user running this agent, on their own phone, through the Free Mobile " +
	"notification API: there is no recipient to choose, the account owning the credentials is the only one it can " +
	"reach. Use it to notify them, e.g. when a long task finishes or you need their input. The message is sent " +
	"untouched. A rate-limit error means too many SMS in too little time: wait before retrying."

// SMS sends a message to the user's phone.
type SMS interface {
	Send(ctx context.Context, msg string) error
}

// Server is the MCP server exposing the send_sms tool.
type Server struct {
	sms     SMS
	version string
}

// NewServer returns a Server reporting version to MCP clients.
func NewServer(sms SMS, version string) *Server {
	return &Server{sms: sms, version: version}
}

type sendSMSArgs struct {
	Message string `json:"message" jsonschema:"the text to send, forwarded untouched"`
}

// Run serves MCP on stdin/stdout until ctx is done or the client disconnects.
func (s *Server) Run(ctx context.Context) error {
	if err := s.mcpServer().Run(ctx, &sdk.StdioTransport{}); err != nil {
		return fmt.Errorf("serving MCP on stdio: %w", err)
	}

	return nil
}

// Serve connects the server to transport; it exists for tests.
func (s *Server) Serve(ctx context.Context, transport sdk.Transport) (*sdk.ServerSession, error) {
	session, err := s.mcpServer().Connect(ctx, transport, nil)
	if err != nil {
		return nil, fmt.Errorf("connecting the MCP server: %w", err)
	}

	return session, nil
}

func (s *Server) mcpServer() *sdk.Server {
	server := sdk.NewServer(
		&sdk.Implementation{Name: "free-mobile-sms-mcp", Version: s.version},
		nil,
	)
	sdk.AddTool(server, &sdk.Tool{Name: "send_sms", Description: sendSMSDescription}, s.sendSMS)

	return server
}

func (s *Server) sendSMS(
	ctx context.Context,
	_ *sdk.CallToolRequest,
	args sendSMSArgs,
) (*sdk.CallToolResult, any, error) {
	if err := s.sms.Send(ctx, args.Message); err != nil {
		//nolint:nilerr // a failed send is a tool result the agent reads, not a protocol error
		return &sdk.CallToolResult{
			IsError: true,
			Content: []sdk.Content{&sdk.TextContent{Text: err.Error()}},
		}, nil, nil
	}

	return &sdk.CallToolResult{
		Content: []sdk.Content{&sdk.TextContent{Text: "SMS sent."}},
	}, nil, nil
}
