package cli

import (
	"fmt"
	"os"
	"strings"

	"github.com/thomiceli/opengist/internal/ipc"
	"github.com/thomiceli/opengist/internal/ssh"
	"github.com/urfave/cli/v2"
)

// CmdKeys is sshd's AuthorizedKeysCommand entry point. For each public key a
// client offers, sshd runs it with the key type and content; if the key is
// known, it prints the matching authorized_keys line (a forced command that
// hands the connection to `opengist shell`).
//
// stdout is parsed by sshd as authorized_keys, so this command writes ONLY the
// key line there — never logs or errors (those go to stderr). The default log
// output includes stdout, so it deliberately avoids the global logger.
//
// Configure sshd with, e.g. (--config is a global flag, so it precedes the
// subcommand):
//
//	AuthorizedKeysCommand /usr/local/bin/opengist --config /etc/opengist/config.yml keys -t %t -k %k
//	AuthorizedKeysCommandUser opengist
var CmdKeys = cli.Command{
	Name:  "keys",
	Usage: "Print the authorized_keys line for an SSH key (sshd AuthorizedKeysCommand)",
	Flags: []cli.Flag{
		&cli.StringFlag{Name: "type", Aliases: []string{"t"}, Usage: "SSH key type, sshd's %t token"},
		&cli.StringFlag{Name: "key", Aliases: []string{"k"}, Usage: "Base64 SSH key, sshd's %k token"},
	},
	Action: func(ctx *cli.Context) error {
		subprocessInitClient(ctx)

		keyType := strings.TrimSpace(ctx.String("type"))
		keyContent := strings.TrimSpace(ctx.String("key"))
		if keyType == "" || keyContent == "" {
			// Nothing to match; sshd treats empty output as "no key".
			return nil
		}
		pubKey := keyType + " " + keyContent

		resp, err := ipc.LookupSSHKey(pubKey)
		if err != nil {
			// Never write to stdout on error: emit no key and report on stderr.
			fmt.Fprintln(os.Stderr, "opengist keys: failed to look up SSH key: "+err.Error())
			return nil
		}
		if !resp.Found {
			return nil
		}

		exe, err := os.Executable()
		if err != nil {
			fmt.Fprintln(os.Stderr, "opengist keys: failed to resolve executable path: "+err.Error())
			return nil
		}

		fmt.Println(ssh.AuthorizedKeysLine(exe, ctx.String("config"), resp.KeyID, pubKey))
		return nil
	},
}
