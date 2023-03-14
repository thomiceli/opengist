package config

import (
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
)

var OpengistVersion = "0.0.1"

var C *config

type config struct {
	OpengistHome  string `yaml:"opengist-home"`
	DBFilename    string `yaml:"db-filename"`
	DisableSignup bool   `yaml:"disable-signup"`
	LogLevel      string `yaml:"log-level"`

	HTTP struct {
		Host   string `yaml:"host"`
		Port   string `yaml:"port"`
		Domain string `yaml:"domain"`
		Git    bool   `yaml:"git-enabled"`
	} `yaml:"http"`

	SSH struct {
		Enabled bool   `yaml:"enabled"`
		Host    string `yaml:"host"`
		Port    string `yaml:"port"`
		Domain  string `yaml:"domain"`
		Keygen  string `yaml:"keygen-executable"`
	} `yaml:"ssh"`
}

func InitConfig(configPath string) error {
	c := &config{}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	c.OpengistHome = filepath.Join(homeDir, ".opengist")
	c.LogLevel = "warn"
	file, err := os.Open(configPath)
	if err != nil {
		return err
	}
	defer file.Close()

	d := yaml.NewDecoder(file)
	if err = d.Decode(&c); err != nil {
		return err
	}
	C = c

	return nil
}

func InitLog() {
	if err := os.MkdirAll(filepath.Join(GetHomeDir(), "log"), 0755); err != nil {
		panic(err)
	}
	file, err := os.OpenFile(filepath.Join(GetHomeDir(), "log", "opengist.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	multi := zerolog.MultiLevelWriter(zerolog.NewConsoleWriter(), file)

	var level zerolog.Level
	level, err = zerolog.ParseLevel(C.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	log.Logger = zerolog.New(multi).Level(level).With().Timestamp().Logger()
}

func GetHomeDir() string {
	absolutePath, _ := filepath.Abs(C.OpengistHome)
	return filepath.Clean(absolutePath)
}
