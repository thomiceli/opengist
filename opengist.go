package main

import (
	"flag"
	"fmt"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/db"
	"github.com/thomiceli/opengist/internal/git"
	"github.com/thomiceli/opengist/internal/index"
	"github.com/thomiceli/opengist/internal/memdb"
	"github.com/thomiceli/opengist/internal/ssh"
	"github.com/thomiceli/opengist/internal/web"
	"os"
	"path/filepath"
)

func initialize() {
	fmt.Println("Opengist v" + config.OpengistVersion)

	configPath := flag.String("config", "", "Path to a config file in YML format")
	flag.Parse()

	if err := config.InitConfig(*configPath); err != nil {
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

func main() {
	initialize()

	go web.NewServer(os.Getenv("OG_DEV") == "1").Start()
	go ssh.Start()

	select {}
}
