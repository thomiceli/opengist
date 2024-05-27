package cli

import (
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/index"
	"github.com/thomiceli/opengist/internal/memdb"
	"github.com/thomiceli/opengist/internal/ssh"
	"github.com/thomiceli/opengist/internal/web"
	"github.com/urfave/cli/v2"
	"os"
	"path"
	"path/filepath"
)

var CmdVersion = cli.Command{
	Name:  "version",
	Usage: "Print the version of Opengist",
	Action: func(c *cli.Context) error {
		fmt.Println("Opengist " + config.OpengistVersion)
		return nil
	},
}

var CmdStart = cli.Command{
	Name:  "start",
	Usage: "Start Opengist server",
	Action: func(ctx *cli.Context) error {
		Initialize(ctx)
		go web.NewServer(os.Getenv("OG_DEV") == "1", path.Join(config.GetHomeDir(), "sessions")).Start()
		go ssh.Start()
		select {}
	},
}

var ConfigFlag = cli.StringFlag{
	Name:    "config",
	Aliases: []string{"c"},
	Usage:   "Path to a config file in YAML format",
}

func App() error {
	app := cli.NewApp()
	app.Name = "Opengist"
	app.Usage = "A self-hosted pastebin powered by Git."
	app.HelpName = "opengist"

	app.Commands = []*cli.Command{&CmdVersion, &CmdStart, &CmdHook, &CmdAdmin}
	app.DefaultCommand = CmdStart.Name
	app.Flags = []cli.Flag{
		&ConfigFlag,
	}
	return app.Run(os.Args)
}

func Initialize(ctx *cli.Context) {
	fmt.Println("Opengist " + config.OpengistVersion)

	if err := config.InitConfig(ctx.String("config"), os.Stdout); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Join(config.GetHomeDir()), 0755); err != nil {
		panic(err)
	}

	config.InitLog()

	gitVersion, err := git.GetGitVersion()
	if err != nil {
		log.Fatal().Err(err).Send()
	}

	if ok, err := config.CheckGitVersion(gitVersion); err != nil {
		log.Fatal().Err(err).Send()
	} else if !ok {
		log.Warn().Msg("Git version may be too old, as Opengist has not been tested prior git version 2.28 and some features would not work. " +
			"Current git version: " + gitVersion)
	}

	homePath := config.GetHomeDir()
	log.Info().Msg("Data directory: " + homePath)

	if err := createSymlink(homePath, ctx.String("config")); err != nil {
		log.Fatal().Err(err).Msg("Failed to create symlinks")
	}

	if err := os.MkdirAll(filepath.Join(homePath, "sessions"), 0755); err != nil {
		log.Fatal().Err(err).Send()
	}
	if err := os.MkdirAll(filepath.Join(homePath, "repos"), 0755); err != nil {
		log.Fatal().Err(err).Send()
	}
	if err := os.MkdirAll(filepath.Join(homePath, "tmp", "repos"), 0755); err != nil {
		log.Fatal().Err(err).Send()
	}
	if err := os.MkdirAll(filepath.Join(homePath, "custom"), 0755); err != nil {
		log.Fatal().Err(err).Send()
	}
	log.Info().Msg("Database file: " + filepath.Join(homePath, config.C.DBFilename))
	if err := db.Setup(filepath.Join(homePath, config.C.DBFilename), false); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}

	if err := memdb.Setup(); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize in memory database")
	}

	if config.C.IndexEnabled {
		log.Info().Msg("Index directory: " + filepath.Join(homePath, config.C.IndexDirname))
		if err := index.Open(filepath.Join(homePath, config.C.IndexDirname)); err != nil {
			log.Fatal().Err(err).Msg("Failed to open index")
		}
	}
}

func createSymlink(homePath string, configPath string) error {
	if err := os.MkdirAll(filepath.Join(homePath, "symlinks"), 0755); err != nil {
		return err
	}

	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	symlinkExePath := path.Join(config.GetHomeDir(), "symlinks", "opengist")
	if _, err := os.Lstat(symlinkExePath); err == nil {
		if err := os.Remove(symlinkExePath); err != nil {
			return err
		}
	}
	if err = os.Symlink(exePath, symlinkExePath); err != nil {
		return err
	}

	if configPath == "" {
		return nil
	}

	configPath, _ = filepath.Abs(configPath)
	configPath = filepath.Clean(configPath)
	symlinkConfigPath := path.Join(config.GetHomeDir(), "symlinks", "config.yml")
	if _, err := os.Lstat(symlinkConfigPath); err == nil {
		if err := os.Remove(symlinkConfigPath); err != nil {
			return err
		}
	}
	if err = os.Symlink(configPath, symlinkConfigPath); err != nil {
		return err
	}

	return nil
}
