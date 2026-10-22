package credentials

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
)

// The environment variables a token command receives.
const (
	EnvUser     = "STORE_CONFORMANCE_USER"
	EnvClientID = "STORE_CONFORMANCE_CLIENT_ID"
)

// CommandSource runs Command with sh -c, EnvUser and EnvClientID set, and
// reads its standard output with Parse.
type CommandSource struct {
	Command string
}

// Fetch implements Source.
func (s CommandSource) Fetch(ctx context.Context, user, client string) (Credentials, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", s.Command)
	cmd.Env = append(os.Environ(), EnvUser+"="+user, EnvClientID+"="+client)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return Credentials{}, fmt.Errorf("credentials: token command: %w: %s", err, bytes.TrimSpace(stderr.Bytes()))
	}
	return Parse(stdout.Bytes(), client)
}
