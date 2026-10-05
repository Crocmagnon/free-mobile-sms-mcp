// Package freemobile sends an SMS through the Free Mobile notification API, which only
// delivers to the line owning the credentials.
package freemobile

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Endpoint is the production API.
const Endpoint = "https://smsapi.free-mobile.fr/sendmsg"

const timeout = 30 * time.Second

//nolint:staticcheck // the brand name starts the sentence
var (
	// ErrBadRequest is HTTP 400.
	ErrBadRequest = errors.New("Free Mobile rejected the request: a required parameter is missing")
	// ErrRateLimited is HTTP 402.
	ErrRateLimited = errors.New(
		"Free Mobile is rate limiting: too many SMS sent in too little time, wait before retrying",
	)
	// ErrForbidden is HTTP 403.
	ErrForbidden = errors.New(
		"Free Mobile refused the credentials: the SMS notification service is not enabled " +
			"in the subscriber area, or the login or key is wrong",
	)
	// ErrServer is HTTP 500.
	ErrServer = errors.New("Free Mobile server error: try again later")
	// ErrUnexpectedStatus is any other status but 200.
	ErrUnexpectedStatus = errors.New("unexpected answer from Free Mobile")
)

// Client sends SMS with one account's credentials.
type Client struct {
	httpClient *http.Client
	endpoint   string
	user       string
	pass       string
}

// NewHTTPClient is the HTTP client to give New outside tests.
func NewHTTPClient() *http.Client {
	return &http.Client{
		Timeout: timeout,
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

// New builds a client posting to endpoint, normally Endpoint.
func New(httpClient *http.Client, endpoint, user, pass string) *Client {
	return &Client{httpClient: httpClient, endpoint: endpoint, user: user, pass: pass}
}

// Send posts msg as a form, so the key stays out of the URL and of any error quoting it.
func (c *Client) Send(ctx context.Context, msg string) error {
	form := url.Values{"user": {c.user}, "pass": {c.pass}, "msg": {msg}}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		c.endpoint,
		strings.NewReader(form.Encode()),
	)
	if err != nil {
		return fmt.Errorf("building the request: %w", err)
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("calling Free Mobile: %w", err)
	}

	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusOK:
		return nil
	case http.StatusBadRequest:
		return ErrBadRequest
	case http.StatusPaymentRequired:
		return ErrRateLimited
	case http.StatusForbidden:
		return ErrForbidden
	case http.StatusInternalServerError:
		return ErrServer
	default:
		return fmt.Errorf("%w: HTTP %d", ErrUnexpectedStatus, resp.StatusCode)
	}
}
