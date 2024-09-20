package cli

import (
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/hooks"
	"github.com/urfave/cli/v2"
	"io"
	"os"
)

var CmdHook = cli.Command{
	Name:  "hook",
	Usage: "Run Git server hooks, used and should only be called by Opengist itself",
	Subcommands: []*cli.Command{
		&CmdHookPreReceive,
		&CmdHookPostReceive,
	},
}

var CmdHookPreReceive = cli.Command{
	Name:  "pre-receive",
	Usage: "Run Git server pre-receive hook for a repository",
	Action: func(ctx *cli.Context) error {
		initialize(ctx)
		if err := hooks.PreReceive(os.Stdin, os.Stdout, os.Stderr); err != nil {
			os.Exit(1)
		}
		return nil
	},
}

var CmdHookPostReceive = cli.Command{
	Name:  "post-receive",
	Usage: "Run Git server post-receive hook for a repository",
	Action: func(ctx *cli.Context) error {
		initialize(ctx)
		if err := hooks.PostReceive(os.Stdin, os.Stdout, os.Stderr); err != nil {
			os.Exit(1)
		}
		return nil
	},
}

func initialize(ctx *cli.Context) {
	if err := config.InitConfig(ctx.String("config"), io.Discard); err != nil {
		panic(err)
	}
	config.InitLog()

	db.DeprecationDBFilename()
	if err := db.Setup(config.C.DBUri, false); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database in hooks")
	}
}
