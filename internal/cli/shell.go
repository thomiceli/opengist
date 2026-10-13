package cli

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/thomiceli/opengist/internal/ipc"
	"github.com/urfave/cli/v2"
)

// CmdShell is the forced command sshd runs after a key matches (it is embedded
// in the authorized_keys line as `opengist shell <keyID>`). It authorizes the
// requested git command against the daemon over the IPC API, then runs the git
// pack command locally, streaming the protocol over stdin/stdout. stdout carries
// the git protocol, so nothing else is written there.
var CmdShell = cli.Command{
	Name:      "shell",
	Usage:     "Serve a single git command over SSH (forced command; called by sshd)",
	ArgsUsage: "[ssh key id]",
	Hidden:    true,
	Action: func(ctx *cli.Context) error {
		subprocessInitClient(ctx)

		code, err := runShell(ctx)
		if err != nil {
			fmt.Fprintln(os.Stderr, "Opengist: "+err.Error())
			os.Exit(1)
		}
		os.Exit(code)
		return nil
	},
}

func runShell(ctx *cli.Context) (int, error) {
	if ctx.NArg() < 1 {
		return 1, errors.New("missing SSH key id")
	}
	keyID, err := strconv.ParseUint(ctx.Args().Get(0), 10, 64)
	if err != nil {
		return 1, errors.New("invalid SSH key id")
	}

	originalCmd := os.Getenv("SSH_ORIGINAL_COMMAND")
	if originalCmd == "" {
		fmt.Fprintln(os.Stderr, "Hi! You've successfully authenticated to Opengist, but Opengist does not provide shell access.")
		return 0, nil
	}

	resp, err := ipc.AuthorizeSSHCommand(&ipc.SSHCommandRequest{
		KeyID:   uint(keyID),
		Command: originalCmd,
		IP:      sshClientIP(),
	})
	if err != nil {
		return 1, err
	}
	if !resp.Authorized {
		return 1, errors.New(resp.Message)
	}

	// Data plane stays local: run the authorized git pack command against the
	// repo on disk. OPENGIST_REPOSITORY_ID lets the post-receive hook update the
	// gist over the IPC API, the same way the HTTP push path does.
	cmd := exec.Command("git", resp.Verb, resp.RepoPath)
	cmd.Dir = resp.RepoPath
	cmd.Env = append(os.Environ(), "OPENGIST_REPOSITORY_ID="+resp.GistID)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		var exitErr *exec.ExitError
		if errors.As(err, &exitErr) {
			return exitErr.ExitCode(), nil
		}
		return 1, err
	}
	return 0, nil
}

// sshClientIP extracts the connecting client's IP from SSH_CONNECTION
// ("<client ip> <client port> <server ip> <server port>"), for logging.
func sshClientIP() string {
	if fields := strings.Fields(os.Getenv("SSH_CONNECTION")); len(fields) > 0 {
		return fields[0]
	}
	return ""
}
