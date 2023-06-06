package config

import (
	"fmt"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"reflect"
	"strconv"
	"strings"
)

var OpengistVersion = "1.3.0"

var C *config

// Not using nested structs because the library
// doesn't support dot notation in this case sadly
type config struct {
	LogLevel     string `yaml:"log-level" env:"OG_LOG_LEVEL"`
	ExternalUrl  string `yaml:"external-url" env:"OG_EXTERNAL_URL"`
	OpengistHome string `yaml:"opengist-home" env:"OG_OPENGIST_HOME"`
	DBFilename   string `yaml:"db-filename" env:"OG_DB_FILENAME"`

	HttpHost       string `yaml:"http.host" env:"OG_HTTP_HOST"`
	HttpPort       string `yaml:"http.port" env:"OG_HTTP_PORT"`
	HttpGit        bool   `yaml:"http.git-enabled" env:"OG_HTTP_GIT_ENABLED"`
	HttpTLSEnabled bool   `yaml:"http.tls-enabled" env:"OG_HTTP_TLS_ENABLED"`
	HttpCertFile   string `yaml:"http.cert-file" env:"OG_HTTP_CERT_FILE"`
	HttpKeyFile    string `yaml:"http.key-file" env:"OG_HTTP_KEY_FILE"`

	SshGit            bool   `yaml:"ssh.git-enabled" env:"OG_SSH_GIT_ENABLED"`
	SshHost           string `yaml:"ssh.host" env:"OG_SSH_HOST"`
	SshPort           string `yaml:"ssh.port" env:"OG_SSH_PORT"`
	SshExternalDomain string `yaml:"ssh.external-domain" env:"OG_SSH_EXTERNAL_DOMAIN"`
	SshKeygen         string `yaml:"ssh.keygen-executable" env:"OG_SSH_KEYGEN_EXECUTABLE"`

	GithubClientKey string `yaml:"github.client-key" env:"OG_GITHUB_CLIENT_KEY"`
	GithubSecret    string `yaml:"github.secret" env:"OG_GITHUB_SECRET"`

	GiteaClientKey string `yaml:"gitea.client-key" env:"OG_GITEA_CLIENT_KEY"`
	GiteaSecret    string `yaml:"gitea.secret" env:"OG_GITEA_SECRET"`
	GiteaUrl       string `yaml:"gitea.url" env:"OG_GITEA_URL"`
}

func configWithDefaults() (*config, error) {
	homeDir, err := os.UserHomeDir()
	c := &config{}
	if err != nil {
		return c, err
	}

	c.LogLevel = "warn"
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

	c.GiteaUrl = "http://gitea.com"

	return c, nil
}

func InitConfig(configPath string) error {
	// Default values
	c, err := configWithDefaults()
	if err != nil {
		return err
	}

	if err = loadConfigFromYaml(c, configPath); err != nil {
		return err
	}

	if err = loadConfigFromEnv(c); err != nil {
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

	var level zerolog.Level
	level, err = zerolog.ParseLevel(C.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	multi := zerolog.MultiLevelWriter(zerolog.NewConsoleWriter(), file)
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

func loadConfigFromYaml(c *config, configPath string) error {
	if configPath != "" {
		absolutePath, _ := filepath.Abs(configPath)
		absolutePath = filepath.Clean(absolutePath)
		file, err := os.Open(absolutePath)
		if err != nil {
			if !os.IsNotExist(err) {
				return err
			}
			fmt.Println("No YAML config file found at " + absolutePath)
		} else {
			fmt.Println("Using YAML config file: " + absolutePath)

			// Override default values with values from config.yml
			d := yaml.NewDecoder(file)
			if err = d.Decode(&c); err != nil {
				return err
			}
			defer file.Close()
		}
	} else {
		fmt.Println("No YAML config file specified.")
	}

	// Override default values with environment variables (as yaml)
	configEnv := os.Getenv("CONFIG")
	if configEnv != "" {
		fmt.Println("Using config from environment variable: CONFIG")
		d := yaml.NewDecoder(strings.NewReader(configEnv))
		if err := d.Decode(&c); err != nil {
			return err
		}
	}

	return nil
}

func loadConfigFromEnv(c *config) error {
	v := reflect.ValueOf(c).Elem()
	var envVars []string

	for i := 0; i < v.NumField(); i++ {
		tag := v.Type().Field(i).Tag.Get("env")

		if tag == "" {
			continue
		}

		envValue := os.Getenv(strings.ToUpper(tag))
		if envValue == "" {
			continue
		}

		switch v.Field(i).Kind() {
		case reflect.String:
			v.Field(i).SetString(envValue)
		case reflect.Bool:
			boolVal, err := strconv.ParseBool(envValue)
			if err != nil {
				return err
			}
			v.Field(i).SetBool(boolVal)
		}

		envVars = append(envVars, tag)
	}

	if len(envVars) > 0 {
		fmt.Println("Using environment variables config: " + strings.Join(envVars, ", "))
	} else {
		fmt.Println("No environment variables config specified.")
	}

	return nil
}
