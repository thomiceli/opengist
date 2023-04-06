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

// Not using nested structs because the library
// doesn't support dot notation in this case sadly
type config struct {
	LogLevel      string `yaml:"log-level"`
	ExternalUrl   string `yaml:"external-url"`
	DisableSignup bool   `yaml:"disable-signup"`
	OpengistHome  string `yaml:"opengist-home"`
	DBFilename    string `yaml:"db-filename"`

	HttpHost       string `yaml:"http.host"`
	HttpPort       string `yaml:"http.port"`
	HttpGit        bool   `yaml:"http.git-enabled"`
	HttpTLSEnabled bool   `yaml:"http.tls-enabled"`
	HttpCertFile   string `yaml:"http.cert-file"`
	HttpKeyFile    string `yaml:"http.key-file"`

	SshGit            bool   `yaml:"ssh.git-enabled"`
	SshHost           string `yaml:"ssh.host"`
	SshPort           string `yaml:"ssh.port"`
	SshExternalDomain string `yaml:"ssh.external-domain"`
	SshKeygen         string `yaml:"ssh.keygen-executable"`
}

func configWithDefaults() (*config, error) {
	homeDir, err := os.UserHomeDir()
	c := &config{}
	if err != nil {
		return c, err
	}

	c.LogLevel = "warn"
	c.DisableSignup = false
	c.OpengistHome = filepath.Join(homeDir, ".opengist")
	c.DBFilename = "opengist.db"

	c.HttpHost = "0.0.0.0"
	c.HttpPort = "6157"
	c.HttpGit = true
	c.HttpTLSEnabled = false

	c.SshGit = true
	c.SshHost = "0.0.0.0"
	c.SshPort = "2222"
	c.SshKeygen = "ssh-keygen"

	return c, nil
}

func InitConfig(configPath string) error {
	// Default values
	c, err := configWithDefaults()
	if err != nil {
		return err
	}

	file, err := os.Open(configPath)
	if err == nil {
		fmt.Println("Using config file: " + configPath)

		// Override default values with values from config.yml
		d := yaml.NewDecoder(file)
		if err = d.Decode(&c); err != nil {
			return err
		}
		defer file.Close()
	}

	// Override default values with environment variables (as yaml)
	configEnv := os.Getenv("CONFIG")
	if configEnv != "" {
		fmt.Println("Using config from environment variable: CONFIG")
		d := yaml.NewDecoder(strings.NewReader(configEnv))
		if err = d.Decode(&c); err != nil {
			return err
		}
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
