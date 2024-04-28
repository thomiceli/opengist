package i18n

import (
	"fmt"
	"github.com/thomiceli/opengist/internal/i18n/locales"
	"golang.org/x/text/cases"
	"golang.org/x/text/language"
	"golang.org/x/text/language/display"
	"gopkg.in/yaml.v3"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

var title = cases.Title(language.English)
var Locales = NewLocaleStore()

type LocaleStore struct {
	Locales map[string]*Locale
}

type Locale struct {
	Code     string
	Name     string
	Messages map[string]string
}

// NewLocaleStore creates a new LocaleStore
func NewLocaleStore() *LocaleStore {
	return &LocaleStore{
		Locales: make(map[string]*Locale),
	}
}

// loadLocaleFromYAML loads a single Locale from a given YAML file
func (store *LocaleStore) loadLocaleFromYAML(localeCode, path string) error {
	a, err := locales.Files.Open(path)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(a)
	if err != nil {
		return err
	}

	tag, err := language.Parse(localeCode)
	if err != nil {
		return err
	}

	name := display.Self.Name(tag)
	if tag == language.AmericanEnglish {
		name = "English"
	} else if tag == language.EuropeanSpanish {
		name = "Español"
	}

	locale := &Locale{
		Code:     localeCode,
		Name:     title.String(name),
		Messages: make(map[string]string),
	}

	err = yaml.Unmarshal(data, &locale.Messages)
	if err != nil {
		return err
	}

	store.Locales[localeCode] = locale
	return nil
}

func (store *LocaleStore) LoadAll() error {
	return fs.WalkDir(locales.Files, ".", func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() {
			localeKey := strings.TrimSuffix(path, filepath.Ext(path))
			err := store.loadLocaleFromYAML(localeKey, path)
			if err != nil {
				return err
			}
		}
		return nil
	})
}

func (store *LocaleStore) GetLocale(lang string) (*Locale, error) {
	_, ok := store.Locales[lang]
	if !ok {
		return nil, fmt.Errorf("locale %s not found", lang)
	}

	return store.Locales[lang], nil
}

func (store *LocaleStore) HasLocale(lang string) bool {
	_, ok := store.Locales[lang]
	return ok
}

func (store *LocaleStore) MatchTag(langs []language.Tag) string {
	for _, lang := range langs {
		if store.HasLocale(lang.String()) {
			return lang.String()
		}
	}

	return "en-US"
}

func (l *Locale) String(key string, args ...any) string {
	message := l.Messages[key]

	if message == "" {
		return Locales.Locales["en-US"].String(key, args...)
	}

	if len(args) == 0 {
		return message
	}

	return fmt.Sprintf(message, args...)
}

func (l *Locale) Tr(key string, args ...any) template.HTML {
	message := l.Messages[key]

	if message == "" {
		return Locales.Locales["en-US"].Tr(key, args...)
	}

	if len(args) == 0 {
		return template.HTML(message)
	}

	return template.HTML(fmt.Sprintf(message, args...))
}
