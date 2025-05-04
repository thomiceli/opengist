package db

import (
	"errors"
	"fmt"
	"github.com/glebarez/sqlite"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/gorm/logger"
	"net/url"
	"path/filepath"
	"slices"
	"strings"
	"time"

	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"gorm.io/gorm"
)

var db *gorm.DB

const (
	SQLite databaseType = iota
	PostgreSQL
	MySQL
)

type databaseType int

func (d databaseType) String() string {
	return [...]string{"SQLite", "PostgreSQL", "MySQL"}[d]
}

type databaseInfo struct {
	Type     databaseType
	Host     string
	Port     string
	User     string
	Password string
	Database string
	SSLMode  string
}

var DatabaseInfo *databaseInfo

func parseDBURI(uri string) (*databaseInfo, error) {
	info := &databaseInfo{}

	info.SSLMode = "disable"

	if uri == ":memory:" {
		info.Type = SQLite
		info.Database = uri
		return info, nil
	}

	u, err := url.Parse(uri)
	if err != nil {
		return nil, fmt.Errorf("invalid URI: %v", err)
	}

	if u.Scheme == "" {
		info.Type = SQLite
		info.Database = filepath.Join(config.GetHomeDir(), uri)
		return info, nil
	}

	switch u.Scheme {
	case "postgres", "postgresql":
		info.Type = PostgreSQL
	case "mysql", "mariadb":
		info.Type = MySQL
	case "file":
		info.Type = SQLite
	default:
		return nil, fmt.Errorf("unknown database: %v", err)
	}

	if u.Host != "" {
		host, port, _ := strings.Cut(u.Host, ":")
		info.Host = host
		info.Port = port
	}

	if u.User != nil {
		info.User = u.User.Username()
		info.Password, _ = u.User.Password()
	}

	if u.RawQuery != "" {
		q, _ := url.ParseQuery(u.RawQuery)
		if sslmode := q.Get("sslmode"); sslmode != "" && info.Type == PostgreSQL {
			info.SSLMode = sslmode
		}
	}

	switch info.Type {
	case PostgreSQL, MySQL:
		info.Database = strings.TrimPrefix(u.Path, "/")
	case SQLite:
		info.Database = u.String()
	default:
		return nil, fmt.Errorf("unknown database: %v", err)
	}

	return info, nil
}

func Setup(dbUri string) error {
	dbInfo, err := parseDBURI(dbUri)
	if err != nil {
		return err
	}

	log.Info().Msgf("Setting up a %s database connection", dbInfo.Type)
	var setupFunc func(databaseInfo) error
	switch dbInfo.Type {
	case SQLite:
		setupFunc = setupSQLite
	case PostgreSQL:
		setupFunc = setupPostgres
	case MySQL:
		setupFunc = setupMySQL
	default:
		return fmt.Errorf("unknown database type: %v", dbInfo.Type)
	}

	maxAttempts := 60
	retryInterval := 1 * time.Second

	for attempt := 1; attempt <= maxAttempts; attempt++ {
		err = setupFunc(*dbInfo)
		if err == nil {
			log.Info().Msg("Database connection established")
			break
		}

		if attempt < maxAttempts {
			log.Warn().Err(err).Msgf("Failed to connect to database (attempt %d), retrying in %v...", attempt, retryInterval)
			time.Sleep(retryInterval)
		} else {
			return err
		}
	}

	DatabaseInfo = dbInfo

	if err = db.SetupJoinTable(&Gist{}, "Likes", &Like{}); err != nil {
		return err
	}

	if err = db.SetupJoinTable(&User{}, "Liked", &Like{}); err != nil {
		return err
	}

	if err = db.AutoMigrate(&User{}, &Gist{}, &SSHKey{}, &AdminSetting{}, &Invitation{}, &WebAuthnCredential{}, &TOTP{}, &GistTopic{}, &GistLanguage{}); err != nil {
		return err
	}

	if err = applyMigrations(dbInfo); err != nil {
		return err
	}

	// Default admin setting values
	return initAdminSettings(map[string]string{
		SettingDisableSignup:          "0",
		SettingRequireLogin:           "0",
		SettingAllowGistsWithoutLogin: "0",
		SettingDisableLoginForm:       "0",
		SettingDisableGravatar:        "0",
	})
}

func Close() error {
	sqlDB, err := db.DB()
	if err != nil {
		return err
	}
	return sqlDB.Close()
}

func CountAll(table interface{}) (int64, error) {
	var count int64
	err := db.Model(table).Count(&count).Error
	return count, err
}

func IsUniqueConstraintViolation(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey)
}

func Ping() error {
	sql, err := db.DB()
	if err != nil {
		return err
	}

	return sql.Ping()
}

func setupSQLite(dbInfo databaseInfo) error {
	var err error
	var dsn string
	journalMode := strings.ToUpper(config.C.SqliteJournalMode)

	if !slices.Contains([]string{"DELETE", "TRUNCATE", "PERSIST", "MEMORY", "WAL", "OFF"}, journalMode) {
		log.Warn().Msg("Invalid SQLite journal mode: " + journalMode)
	}

	if dbInfo.Database == ":memory:" {
		dsn = ":memory:?_fk=true&cache=shared"
	} else {
		u, err := url.Parse(dbInfo.Database)
		if err != nil {
			return err
		}

		u.Scheme = "file"
		q := u.Query()
		q.Set("_pragma", "foreign_keys(1)")
		q.Set("_journal_mode", journalMode)
		u.RawQuery = q.Encode()
		dsn = u.String()
	}
	db, err = gorm.Open(sqlite.Open(dsn), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
	})
	return err
}

func setupPostgres(dbInfo databaseInfo) error {
	var err error
	dsn := fmt.Sprintf("host=%s port=%s user=%s password=%s dbname=%s sslmode=%s", dbInfo.Host, dbInfo.Port, dbInfo.User, dbInfo.Password, dbInfo.Database, dbInfo.SSLMode)

	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
	})

	return err
}

func setupMySQL(dbInfo databaseInfo) error {
	var err error
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local", dbInfo.User, dbInfo.Password, dbInfo.Host, dbInfo.Port, dbInfo.Database)

	db, err = gorm.Open(mysql.New(mysql.Config{
		DSN:                    dsn,
		DontSupportRenameIndex: true,
	}), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Silent),
		TranslateError: true,
	})

	return err
}

func DeprecationDBFilename() {
	if config.C.DBFilename != "" {
		log.Warn().Msg("The 'db-filename'/'OG_DB_FILENAME' configuration option is deprecated and will be removed in a future version. Please use 'db-uri'/'OG_DB_URI' instead.")
	}

	if config.C.DBUri == "" {
		config.C.DBUri = config.C.DBFilename
	}
}

func TruncateDatabase() error {
	return db.Migrator().DropTable("likes", &User{}, "gists", &SSHKey{}, &AdminSetting{}, &Invitation{}, &WebAuthnCredential{}, &TOTP{}, &GistTopic{}, &GistLanguage{})
}
