// Command free-mobile-sms-mcp is an MCP server that lets a coding agent text the user driving it,
// plus the subcommands that user runs to store the Free Mobile credentials it needs.
package main

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"runtime/debug"
	"strings"
	"syscall"

	"golang.org/x/term"

	"go.augendre.info/free-mobile-sms-mcp/adapters/credentials"
	"go.augendre.info/free-mobile-sms-mcp/adapters/freemobile"
	mcpadapter "go.augendre.info/free-mobile-sms-mcp/adapters/mcp"
	"go.augendre.info/free-mobile-sms-mcp/domain/sms"
)

const (
	devVersion = "(devel)"
	usage      = `usage: free-mobile-sms-mcp [serve|login|logout|version]

  serve    run the MCP server on stdio (default)
  login    store the Free Mobile login and API key (interactive terminal only)
  logout   delete the stored credentials
  version  print the build version
`
	loginHint = "Your login and API key are in the Free Mobile subscriber area " +
		"(https://mobile.free.fr/account/), under Mes Options > Notifications par SMS: " +
		"enable the option first."
)

var (
	errNotATerminal = errors.New(
		"stdin is not a terminal: run `free-mobile-sms-mcp login` yourself, in your own terminal",
	)
	errNothingEntered = errors.New("nothing entered")
	errUnknownCommand = errors.New("unknown command")
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run() error {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	store := credentials.New()

	switch cmd {
	case "serve":
		return serve(ctx, store)
	case "login":
		return login(ctx, store)
	case "logout":
		return logout(store)
	case "version":
		_, _ = fmt.Fprintln(os.Stdout, version())

		return nil
	case "-h", "--help", "help":
		_, _ = fmt.Fprint(os.Stdout, usage)

		return nil
	default:
		fmt.Fprint(os.Stderr, usage)

		return fmt.Errorf("%w %q", errUnknownCommand, cmd)
	}
}

func serve(ctx context.Context, store credentials.Store) error {
	creds, err := store.Get()
	if err != nil {
		return err //nolint:wrapcheck // the store's error already says what to do
	}

	client := freemobile.New(
		freemobile.NewHTTPClient(),
		freemobile.Endpoint,
		creds.User,
		creds.Pass,
	)

	if err := mcpadapter.NewServer(sms.NewService(client), version()).Run(ctx); err != nil {
		return fmt.Errorf("serving: %w", err)
	}

	return nil
}

func login(ctx context.Context, store credentials.Store) error {
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		return errNotATerminal
	}

	fmt.Fprintln(os.Stderr, loginHint)

	creds, err := readCredentials(os.Stdin, os.Stderr, func() ([]byte, error) {
		return term.ReadPassword(int(os.Stdin.Fd()))
	})
	if err != nil {
		return err
	}

	client := freemobile.New(
		freemobile.NewHTTPClient(),
		freemobile.Endpoint,
		creds.User,
		creds.Pass,
	)
	if err := client.Send(ctx, "free-mobile-sms-mcp: login OK"); err != nil {
		return fmt.Errorf("checking the credentials with a test SMS: %w", err)
	}

	if err := store.Save(creds); err != nil {
		return fmt.Errorf("saving the credentials: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Saved to %s. You should have received a test SMS.\n", store.Path())

	return nil
}

func logout(store credentials.Store) error {
	if err := store.Delete(); err != nil {
		return fmt.Errorf("removing the credentials: %w", err)
	}

	_, _ = fmt.Fprintf(os.Stdout, "Removed %s.\n", store.Path())

	return nil
}

func readCredentials(
	input io.Reader,
	out io.Writer,
	readSecret func() ([]byte, error),
) (credentials.Credentials, error) {
	_, _ = fmt.Fprint(out, "Free Mobile login: ")

	line, err := bufio.NewReader(input).ReadString('\n')
	if err != nil && !errors.Is(err, io.EOF) {
		return credentials.Credentials{}, fmt.Errorf("reading the login: %w", err)
	}

	user := strings.TrimSpace(line)
	if user == "" {
		return credentials.Credentials{}, errNothingEntered
	}

	_, _ = fmt.Fprint(out, "API key (hidden): ")

	secret, err := readSecret()

	_, _ = fmt.Fprintln(out)

	if err != nil {
		return credentials.Credentials{}, fmt.Errorf("reading the API key: %w", err)
	}

	pass := strings.TrimSpace(string(secret))
	if pass == "" {
		return credentials.Credentials{}, errNothingEntered
	}

	return credentials.Credentials{User: user, Pass: pass}, nil
}

func version() string {
	info, ok := debug.ReadBuildInfo()
	if !ok {
		return "unknown build"
	}

	switch info.Main.Version {
	case devVersion:
		return "dev (working tree)"
	case "":
		return "unknown build"
	default:
		return info.Main.Version
	}
}
