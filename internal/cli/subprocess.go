package cli

import (
	"io"

	"github.com/thomiceli/opengist/internal/config"
	"github.com/urfave/cli/v2"
)

// subprocessInit initializes Opengist for short-lived processes that Opengist
// spawns of itself (Git hooks, the SSH `keys`/`shell` commands). It is the
// shared contract for every self-invoked subcommand:
//
//   - stdout is reserved for the subcommand's own protocol output (git pack
//     stream, authorized_keys lines, hook messages). Config output is discarded,
//     and logging is left at zerolog's default (stderr) — InitLog's console
//     writer goes to stdout, so it is deliberately not called here.
//   - it opens no database: subprocesses talk to the running daemon's internal
//     API instead. Use subprocessInitClient when that API is needed.
func subprocessInit(ctx *cli.Context) {
	if err := config.InitConfig(ctx.String("config"), io.Discard); err != nil {
		panic(err)
	}
}

// subprocessInitClient is subprocessInit plus the secret key, which is needed to
// authenticate calls to the daemon's internal API (see the ipc package).
func subprocessInitClient(ctx *cli.Context) {
	subprocessInit(ctx)
	config.SetupSecretKey()
}
