// Package sms sends a text message to the user driving the agent.
package sms

import (
	"context"
	"errors"
	"fmt"
	"strings"
)

// ErrEmptyMessage says there was nothing to send.
var ErrEmptyMessage = errors.New("the message is empty")

// Sender delivers a message to the user's phone.
type Sender interface {
	Send(ctx context.Context, msg string) error
}

// Service is the send use case.
type Service struct {
	sender Sender
}

// NewService builds the use case on top of a sender.
func NewService(sender Sender) *Service {
	return &Service{sender: sender}
}

// Send forwards msg untouched, unless it is blank.
func (s *Service) Send(ctx context.Context, msg string) error {
	if strings.TrimSpace(msg) == "" {
		return ErrEmptyMessage
	}

	if err := s.sender.Send(ctx, msg); err != nil {
		return fmt.Errorf("sending the SMS: %w", err)
	}

	return nil
}
