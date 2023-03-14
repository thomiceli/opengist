package main

import (
	"flag"
	"github.com/rs/zerolog/log"
	"opengist/internal/config"
	"opengist/internal/models"
	"opengist/internal/ssh"
	"opengist/internal/web"
	"os"
	"path/filepath"
)

func initialize() {
	configPath := flag.String("config", "config.yml", "Path to a config file in YML format")
	flag.Parse()
	absolutePath, _ := filepath.Abs(*configPath)
	absolutePath = filepath.Clean(absolutePath)
	if err := config.InitConfig(absolutePath); err != nil {
		panic(err)
	}
	if err := os.MkdirAll(filepath.Join(config.GetHomeDir()), 0755); err != nil {
		panic(err)
	}

	config.InitLog()

	log.Info().Msg("Opengist v" + config.OpengistVersion)
	log.Info().Msg("Using config file: " + absolutePath)

	homePath := config.GetHomeDir()
	log.Info().Msg("Data directory: " + homePath)

	if err := os.MkdirAll(filepath.Join(homePath, "repos"), 0755); err != nil {
		log.Fatal().Err(err).Send()
	}
	if err := os.MkdirAll(filepath.Join(homePath, "tmp", "repos"), 0755); err != nil {
		log.Fatal().Err(err).Send()
	}

	log.Info().Msg("Database file: " + filepath.Join(homePath, config.C.DBFilename))
	if err := models.Setup(filepath.Join(homePath, config.C.DBFilename)); err != nil {
		log.Fatal().Err(err).Msg("Failed to initialize database")
	}
}

func main() {
	initialize()

	go web.Start()
	go ssh.Start()

	select {}
}
