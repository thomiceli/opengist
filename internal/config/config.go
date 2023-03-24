package config

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

var OpengistVersion = "0.0.1"

var C *config

type config struct {
	OpengistHome  string `yaml:"opengist-home"`
	DBFilename    string `yaml:"db-filename"`
	DisableSignup bool   `yaml:"disable-signup"`
	LogLevel      string `yaml:"log-level"`

	HTTP struct {
		Host       string `yaml:"host"`
		Port       string `yaml:"port"`
		Domain     string `yaml:"domain"`
		Git        bool   `yaml:"git-enabled"`
		TLSEnabled bool   `yaml:"tls-enabled"`
		CertFile   string `yaml:"cert-file"`
		KeyFile    string `yaml:"key-file"`
	} `yaml:"http"`

	SSH struct {
		Enabled bool   `yaml:"enabled"`
		Host    string `yaml:"host"`
		Port    string `yaml:"port"`
		Domain  string `yaml:"domain"`
		Keygen  string `yaml:"keygen-executable"`
	} `yaml:"ssh"`
}

func configWithDefaults() (*config, error) {
	homeDir, err := os.UserHomeDir()
	c := &config{}
	if err != nil {
		return c, err
	}

	c.OpengistHome = filepath.Join(homeDir, ".opengist")
	c.DBFilename = "opengist.db"
	c.DisableSignup = false
	c.LogLevel = "warn"

	c.HTTP.Host = "0.0.0.0"
	c.HTTP.Port = "6157"
	c.HTTP.Domain = "localhost"
	c.HTTP.Git = true

	c.HTTP.TLSEnabled = false

	c.SSH.Enabled = true
	c.SSH.Host = "0.0.0.0"
	c.SSH.Port = "2222"
	c.SSH.Domain = "localhost"
	c.SSH.Keygen = "ssh-keygen"

	return c, nil
}

func InitConfig(configPath string) error {
	c, err := configWithDefaults()
	if err != nil {
		return err
	}

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

func CheckGitVersion(version string) (bool, error) {
	versionParts := strings.Split(version, ".")
	if len(versionParts) < 2 {
		return false, fmt.Errorf("invalid version string")
	}
	major, err := strconv.Atoi(versionParts[0])
	if err != nil {
		return false, fmt.Errorf("invalid major version number")
	}
	minor, err := strconv.Atoi(versionParts[1])
	if err != nil {
		return false, fmt.Errorf("invalid minor version number")
	}

	// Check if version is prior to 2.20
	if major < 2 || (major == 2 && minor < 20) {
		return false, nil
	}
	return true, nil
}

func GetHomeDir() string {
	absolutePath, _ := filepath.Abs(C.OpengistHome)
	return filepath.Clean(absolutePath)
}
