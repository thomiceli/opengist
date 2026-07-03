package config

import (
	"bufio"
	"fmt"
	"io"
	"net/url"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/thomiceli/opengist/internal/session"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/yaml.v3"
)

var OpengistVersion = ""

var C *config

var SecretKey []byte

// startupMessages buffers informational messages produced during config loading
// (before the logger is configured). Call FlushStartupLog after InitLog to
// replay them through the configured logger.
var startupMessages []string

// AddStartupMsg records a message to be logged once the logger is configured.
func AddStartupMsg(msg string) {
	startupMessages = append(startupMessages, msg)
}

// FlushStartupLog replays all buffered startup messages through the configured
// logger. It should be called after InitLog.
func FlushStartupLog() {
	for _, msg := range startupMessages {
		log.Info().Msg(msg)
	}
	startupMessages = nil
}

// SSH server modes: the canonical values of ssh.git-enabled. The legacy
// booleans are still accepted (true → builtin, false → disabled).
const (
	SshServerBuiltin  = "builtin"  // Opengist's embedded SSH server
	SshServerHost     = "host"     // delegate to the host's OpenSSH
	SshServerDisabled = "disabled" // no SSH git access
)

// Not using nested structs because the library
// doesn't support dot notation in this case sadly
type config struct {
	SecretKey string `yaml:"secret-key" env:"OG_SECRET_KEY"`

	LogLevel     string `yaml:"log-level" env:"OG_LOG_LEVEL"`
	LogOutput    string `yaml:"log-output" env:"OG_LOG_OUTPUT"`
	LogPath      string `yaml:"log-path" env:"OG_LOG_PATH"`
	ExternalUrl  string `yaml:"external-url" env:"OG_EXTERNAL_URL"`
	OpengistHome string `yaml:"opengist-home" env:"OG_OPENGIST_HOME"`

	DBUri      string `yaml:"db-uri" env:"OG_DB_URI"`
	DBFilename string `yaml:"db-filename" env:"OG_DB_FILENAME"` // deprecated

	IndexEnabled  bool   `yaml:"index.enabled" env:"OG_INDEX_ENABLED"` // deprecated
	Index         string `yaml:"index" env:"OG_INDEX"`
	BleveDirname  string `yaml:"index.dirname" env:"OG_INDEX_DIRNAME"` // deprecated
	MeiliHost     string `yaml:"index.meili.host" env:"OG_MEILI_HOST"`
	MeiliAPIKey   string `yaml:"index.meili.api-key" env:"OG_MEILI_API_KEY"`
	SearchDefault string `yaml:"search.default" env:"OG_SEARCH_DEFAULT"`

	GitDefaultBranch string `yaml:"git.default-branch" env:"OG_GIT_DEFAULT_BRANCH"`

	SqliteJournalMode string `yaml:"sqlite.journal-mode" env:"OG_SQLITE_JOURNAL_MODE"`

	HttpHost string `yaml:"http.host" env:"OG_HTTP_HOST"`
	HttpPort string `yaml:"http.port" env:"OG_HTTP_PORT"`
	HttpGit  bool   `yaml:"http.git-enabled" env:"OG_HTTP_GIT_ENABLED"`

	ApiEnabled bool `yaml:"api.enabled" env:"OG_API_ENABLED"`

	DisableFileUpload bool `yaml:"disable-file-upload" env:"OG_DISABLE_FILE_UPLOAD"`

	UnixSocketPermissions string `yaml:"unix-socket-permissions" env:"OG_UNIX_SOCKET_PERMISSIONS"`

	SshGit                string `yaml:"ssh.git-enabled" env:"OG_SSH_GIT_ENABLED"` // builtin | host | disabled (true → builtin, false → disabled)
	SshAuthorizedKeysFile string `yaml:"ssh.authorized-keys-file" env:"OG_SSH_AUTHORIZED_KEYS_FILE"`
	SshHost               string `yaml:"ssh.host" env:"OG_SSH_HOST"`
	SshPort               string `yaml:"ssh.port" env:"OG_SSH_PORT"`
	SshExternalDomain     string `yaml:"ssh.external-domain" env:"OG_SSH_EXTERNAL_DOMAIN"`
	SshUsername           string `yaml:"ssh.username" env:"OG_SSH_USERNAME"`

	GithubClientKey string `yaml:"github.client-key" env:"OG_GITHUB_CLIENT_KEY"`
	GithubSecret    string `yaml:"github.secret" env:"OG_GITHUB_SECRET"`

	GitlabClientKey string `yaml:"gitlab.client-key" env:"OG_GITLAB_CLIENT_KEY"`
	GitlabSecret    string `yaml:"gitlab.secret" env:"OG_GITLAB_SECRET"`
	GitlabUrl       string `yaml:"gitlab.url" env:"OG_GITLAB_URL"`
	GitlabName      string `yaml:"gitlab.name" env:"OG_GITLAB_NAME"`

	GiteaClientKey string `yaml:"gitea.client-key" env:"OG_GITEA_CLIENT_KEY"`
	GiteaSecret    string `yaml:"gitea.secret" env:"OG_GITEA_SECRET"`
	GiteaUrl       string `yaml:"gitea.url" env:"OG_GITEA_URL"`
	GiteaName      string `yaml:"gitea.name" env:"OG_GITEA_NAME"`

	OIDCProviderName   string `yaml:"oidc.provider-name" env:"OG_OIDC_PROVIDER_NAME"`
	OIDCClientKey      string `yaml:"oidc.client-key" env:"OG_OIDC_CLIENT_KEY"`
	OIDCSecret         string `yaml:"oidc.secret" env:"OG_OIDC_SECRET"`
	OIDCDiscoveryUrl   string `yaml:"oidc.discovery-url" env:"OG_OIDC_DISCOVERY_URL"`
	OIDCGroupClaimName string `yaml:"oidc.group-claim-name" env:"OG_OIDC_GROUP_CLAIM_NAME"`
	OIDCAdminGroup     string `yaml:"oidc.admin-group" env:"OG_OIDC_ADMIN_GROUP"`

	MetricsEnabled bool   `yaml:"metrics.enabled" env:"OG_METRICS_ENABLED"`
	MetricsHost    string `yaml:"metrics.host" env:"OG_METRICS_HOST"`
	MetricsPort    string `yaml:"metrics.port" env:"OG_METRICS_PORT"`

	LDAPUrl             string `yaml:"ldap.url" env:"OG_LDAP_URL"`
	LDAPBindDn          string `yaml:"ldap.bind-dn" env:"OG_LDAP_BIND_DN"`
	LDAPBindCredentials string `yaml:"ldap.bind-credentials" env:"OG_LDAP_BIND_CREDENTIALS"`
	LDAPSearchBase      string `yaml:"ldap.search-base" env:"OG_LDAP_SEARCH_BASE"`
	LDAPSearchFilter    string `yaml:"ldap.search-filter" env:"OG_LDAP_SEARCH_FILTER"`

	CustomName    string       `yaml:"custom.name" env:"OG_CUSTOM_NAME"`
	CustomLogo    string       `yaml:"custom.logo" env:"OG_CUSTOM_LOGO"`
	CustomFavicon string       `yaml:"custom.favicon" env:"OG_CUSTOM_FAVICON"`
	StaticLinks   []StaticLink `yaml:"custom.static-links" env:"OG_CUSTOM_STATIC_LINK"`
}

type StaticLink struct {
	Name string `yaml:"name" env:"OG_CUSTOM_STATIC_LINK_#_NAME"`
	Path string `yaml:"path" env:"OG_CUSTOM_STATIC_LINK_#_PATH"`
}

// SshEnabled reports whether SSH git access is offered in any form.
func (c *config) SshEnabled() bool {
	return c.SshGit == SshServerBuiltin || c.SshGit == SshServerHost
}

// SshBuiltin reports whether Opengist runs its own embedded SSH server.
func (c *config) SshBuiltin() bool {
	return c.SshGit == SshServerBuiltin
}

// SshManagesAuthorizedKeys reports whether Opengist maintains an authorized_keys
// file: host mode with a configured file path.
func (c *config) SshManagesAuthorizedKeys() bool {
	return c.SshGit == SshServerHost && c.SshAuthorizedKeysFile != ""
}

func configWithDefaults() (*config, error) {
	c := &config{}

	c.SecretKey = ""

	c.LogLevel = "warn"
	c.LogOutput = "stdout,file"
	c.OpengistHome = ""
	c.DBUri = "opengist.db"
	c.Index = "bleve"
	c.SearchDefault = "content"

	c.SqliteJournalMode = "WAL"

	c.HttpHost = "0.0.0.0"
	c.HttpPort = "6157"
	c.HttpGit = true

	c.ApiEnabled = true

	c.UnixSocketPermissions = "0666"

	c.SshGit = SshServerBuiltin
	c.SshHost = "0.0.0.0"
	c.SshPort = "2222"

	c.GitlabName = "GitLab"

	c.GiteaUrl = "https://gitea.com"
	c.GiteaName = "Gitea"

	c.MetricsEnabled = false
	c.MetricsHost = "0.0.0.0"
	c.MetricsPort = "6158"

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

	// Source Docker secrets (a dotenv-style file) into the environment before
	// reading OG_* variables. Explicit environment variables take precedence.
	if err = loadSecretsFile(); err != nil {
		return err
	}

	if err = loadConfigFromEnv(c); err != nil {
		return err
	}

	// ssh.git-enabled accepts the explicit modes (builtin, host, disabled) as well
	// as the legacy booleans (true → builtin, false → disabled). Collapse whatever
	// was provided into a canonical mode.
	c.SshGit = normalizeSshGitMode(c.SshGit)

	if c.OpengistHome == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("opengist home directory is not set and current user home directory could not be determined; please specify the opengist home directory manually via the configuration")
		}

		c.OpengistHome = filepath.Join(homeDir, ".opengist")
	}

	if err = checks(c); err != nil {
		return err
	}

	C = c

	if c.LogPath == "" {
		c.LogPath = filepath.Join(GetHomeDir(), "log")
	}

	if err = migrateConfig(); err != nil {
		return err
	}

	if err = os.Setenv("OG_OPENGIST_HOME_INTERNAL", GetHomeDir()); err != nil {
		return err
	}
	return nil
}

func InitLog() {
	if err := os.MkdirAll(C.LogPath, 0755); err != nil {
		panic(err)
	}

	var level zerolog.Level
	level, err := zerolog.ParseLevel(C.LogLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}

	var logWriters []io.Writer
	logOutputTypes := strings.Split(strings.ToLower(C.LogOutput), ",")
	slices.Sort(logOutputTypes)
	logOutputTypes = slices.Compact(logOutputTypes)

	consoleWriter := zerolog.NewConsoleWriter(
		func(w *zerolog.ConsoleWriter) {
			w.TimeFormat = time.TimeOnly
			w.FormatCaller = func(i interface{}) string {
				file := i.(string)
				index := strings.Index(file, "internal")
				if index == -1 {
					return file
				}
				return file[index:]
			}
		},
	)

	for _, logOutputType := range logOutputTypes {
		logOutputType = strings.TrimSpace(logOutputType)
		if !slices.Contains([]string{"stdout", "file"}, logOutputType) {
			defer func() { log.Warn().Msg("Invalid log output type: " + logOutputType) }()
			continue
		}

		switch logOutputType {
		case "stdout":
			logWriters = append(logWriters, consoleWriter)
			defer func() { log.Debug().Msg("Logging to stdout") }()
		case "file":
			file, err := os.OpenFile(filepath.Join(C.LogPath, "opengist.log"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
			if err != nil {
				panic(err)
			}
			logWriters = append(logWriters, file)
			defer func() { log.Debug().Msg("Logging to file: " + file.Name()) }()
		}
	}
	if len(logWriters) == 0 {
		logWriters = append(logWriters, consoleWriter)
		defer func() { log.Warn().Msg("No valid log outputs, defaulting to stdout") }()
	}

	multi := zerolog.MultiLevelWriter(logWriters...)
	log.Logger = zerolog.New(multi).Level(level).With().Caller().Timestamp().Logger()

	if !slices.Contains([]string{"debug", "info", "warn", "error", "fatal"}, strings.ToLower(C.LogLevel)) {
		log.Warn().Msg("Invalid log level: " + C.LogLevel)
	}
}

// gitVersionRegex extracts the major and minor numbers from a git version string.
// It tolerates a leading "git version " prefix as well as suffixes such as the
// ".windows.1" appended by Git for Windows (e.g. "git version 2.50.1.windows.1").
var gitVersionRegex = regexp.MustCompile(`(\d+)\.(\d+)`)

func CheckGitVersion(version string) (bool, error) {
	matches := gitVersionRegex.FindStringSubmatch(version)
	if matches == nil {
		return false, fmt.Errorf("invalid version string: %q", version)
	}

	major, err := strconv.Atoi(matches[1])
	if err != nil {
		return false, fmt.Errorf("invalid major version number")
	}
	minor, err := strconv.Atoi(matches[2])
	if err != nil {
		return false, fmt.Errorf("invalid minor version number")
	}

	// Check if version is prior to 2.28
	if major < 2 || (major == 2 && minor < 28) {
		return false, nil
	}
	return true, nil
}

func GetHomeDir() string {
	absolutePath, _ := filepath.Abs(C.OpengistHome)
	return filepath.Clean(absolutePath)
}

func SetupSecretKey() {
	if C.SecretKey == "" {
		path := filepath.Join(GetHomeDir(), "opengist-secret.key")
		SecretKey, _ = session.GenerateSecretKey(path)
	} else {
		SecretKey = []byte(C.SecretKey)
	}
}

// defaultSecretsFile is the path Docker Compose/Swarm mounts the secrets file
// at (see docs/configuration/configure.md). It can be overridden via the
// OG_SECRETS_FILE environment variable.
const defaultSecretsFile = "/run/secrets/opengist_secrets"

// loadSecretsFile sources a dotenv-style file (as mounted by Docker
// Compose/Swarm secrets) into the process environment, so that its values can
// be picked up by loadConfigFromEnv like any other OG_* variable. It is a
// no-op when the file is absent, which keeps non-containerized deployments
// unaffected.
//
// Variables already present in the environment are left untouched, so explicit
// configuration always takes precedence over the secrets file.
func loadSecretsFile() error {
	path := os.Getenv("OG_SECRETS_FILE")
	if path == "" {
		path = defaultSecretsFile
	}

	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	defer file.Close()

	info, err := file.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return nil
	}

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		key, value, ok := parseSecretLine(scanner.Text())
		if !ok {
			continue
		}
		if _, isSet := os.LookupEnv(key); !isSet {
			if err := os.Setenv(key, value); err != nil {
				return err
			}
		}
	}

	return scanner.Err()
}

// parseSecretLine parses a single dotenv-style line into a key/value pair. It
// supports an optional "export " prefix, blank lines and comments (#), and
// strips a single layer of surrounding single or double quotes. ok is false
// for lines that do not define a variable.
func parseSecretLine(line string) (key, value string, ok bool) {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}

	line = strings.TrimPrefix(line, "export ")
	key, value, found := strings.Cut(line, "=")
	if !found {
		return "", "", false
	}

	key = strings.TrimSpace(key)
	if key == "" {
		return "", "", false
	}

	value = strings.TrimSpace(value)
	if n := len(value); n >= 2 {
		first, last := value[0], value[n-1]
		if (first == '"' && last == '"') || (first == '\'' && last == '\'') {
			value = value[1 : n-1]
		}
	}

	return key, value, true
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
			AddStartupMsg("No YAML config file found at " + absolutePath)
		} else {
			AddStartupMsg("Using YAML config file: " + absolutePath)

			// Override default values with values from config.yml
			d := yaml.NewDecoder(file)
			if err = d.Decode(&c); err != nil {
				return err
			}
			defer file.Close()
		}
	} else {
		AddStartupMsg("No YAML config file specified.")
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
		if envValue == "" && v.Field(i).Kind() != reflect.Slice {
			continue
		}

		switch v.Field(i).Kind() {
		case reflect.String:
			v.Field(i).SetString(envValue)
			envVars = append(envVars, tag)
		case reflect.Bool:
			boolVal, err := strconv.ParseBool(envValue)
			if err != nil {
				return err
			}
			v.Field(i).SetBool(boolVal)
			envVars = append(envVars, tag)
		case reflect.Slice:
			if v.Type().Field(i).Type.Elem().Kind() == reflect.Struct {
				prefix := strings.ToUpper(tag) + "_"
				var sliceValue reflect.Value
				elemType := v.Type().Field(i).Type.Elem()

				for index := 0; ; index++ {
					allFieldsPresent := true
					elemValue := reflect.New(elemType).Elem()

					for j := 0; j < elemValue.NumField() && allFieldsPresent; j++ {
						elemField := elemValue.Type().Field(j)
						envName := fmt.Sprintf("%s%d_%s", prefix, index, strings.ToUpper(elemField.Name))
						envValue, present := os.LookupEnv(envName)

						if !present {
							allFieldsPresent = false
							break
						}

						envVars = append(envVars, envName)
						elemValue.Field(j).SetString(envValue)
					}

					if !allFieldsPresent {
						break
					}

					if sliceValue.Kind() != reflect.Slice {
						sliceValue = reflect.MakeSlice(v.Type().Field(i).Type, 0, index+1)
					}
					sliceValue = reflect.Append(sliceValue, elemValue)
				}

				if sliceValue.IsValid() {
					v.Field(i).Set(sliceValue)
				}
			}
		default:
			return fmt.Errorf("unsupported type: %s", v.Field(i).Kind())
		}

	}

	if len(envVars) > 0 {
		AddStartupMsg("Using environment variables config: " + strings.Join(envVars, ", "))
	} else {
		AddStartupMsg("No environment variables config specified.")
	}

	return nil
}


// normalizeSshGitMode maps every accepted ssh.git-enabled value to a canonical
// mode. The legacy booleans are preserved for backward compatibility (true →
// builtin, false → disabled); unknown values are returned untouched so checks()
// can reject them.
func normalizeSshGitMode(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "host":
		return SshServerHost
	case "false", "f", "0", "no", "off", "disabled":
		return SshServerDisabled
	case "true", "t", "1", "yes", "on", "builtin", "":
		return SshServerBuiltin
	default:
		return strings.TrimSpace(s)
	}
}

func checks(c *config) error {
	switch c.SshGit {
	case SshServerBuiltin, SshServerHost, SshServerDisabled:
	default:
		return fmt.Errorf("invalid ssh.git-enabled %q (must be %q, %q or %q, or a boolean)", c.SshGit, SshServerBuiltin, SshServerHost, SshServerDisabled)
	}

	if _, err := url.Parse(c.ExternalUrl); err != nil {
		return err
	}

	if _, err := url.Parse(c.GiteaUrl); err != nil {
		return err
	}

	if _, err := url.Parse(c.OIDCDiscoveryUrl); err != nil {
		return err
	}

	return nil
}
