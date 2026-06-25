package cli

import (
	"io"

	"github.com/thomiceli/opengist/internal/config"
	"github.com/urfave/cli/v2"
)

// subprocessInit initializes Opengist for short-lived processes that Opengist
// spawns of itself (Git hooks, and later the SSH shim). It is the shared
// contract for every self-invoked subcommand:
//
//   - stdout is left untouched — config output is discarded — so callers such
//     as the SSH AuthorizedKeysCommand can write only their own payload to it;
//   - logs go through the configured logger (file/stderr), never stdout.
//
// It deliberately does not touch the database: subprocesses talk to the running
// daemon's internal API instead of opening their own connection. Use
// subprocessInitClient when the subprocess needs to call that API.
func subprocessInit(ctx *cli.Context) {
	if err := config.InitConfig(ctx.String("config"), io.Discard); err != nil {
		panic(err)
	}
	config.InitLog()
}

// subprocessInitClient is subprocessInit plus the secret key, which is needed to
// authenticate calls to the daemon's internal API (see the ipc package).
func subprocessInitClient(ctx *cli.Context) {
	subprocessInit(ctx)
	config.SetupSecretKey()
}
