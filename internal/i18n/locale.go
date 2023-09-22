package i18n

import (
	"fmt"
	"github.com/thomiceli/opengist/locales"
	"golang.org/x/text/language"
	"gopkg.in/yaml.v3"
	"html/template"
	"io"
	"io/fs"
	"path/filepath"
	"strings"
)

var Locales = NewLocaleStore()

type Locale struct {
	Name     string
	Messages map[string]string
}

type LocaleStore struct {
	Locales map[string]*Locale
}

// NewLocaleStore creates a new LocaleStore
func NewLocaleStore() *LocaleStore {
	return &LocaleStore{
		Locales: make(map[string]*Locale),
	}
}

// loadLocaleFromYAML loads a single Locale from a given YAML file
func (store *LocaleStore) loadLocaleFromYAML(localeKey, path string) error {
	a, err := locales.Files.Open(path)
	if err != nil {
		return err
	}
	data, err := io.ReadAll(a)
	if err != nil {
		return err
	}

	locale := &Locale{Name: localeKey, Messages: make(map[string]string)}
	err = yaml.Unmarshal(data, &locale.Messages)
	if err != nil {
		return err
	}

	store.Locales[localeKey] = locale
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

// Tr translates a message key into a message for a given locale
func (store *LocaleStore) Tr(localeKey, messageKey string) (string, error) {
	locale, ok := store.Locales[localeKey]
	if !ok {
		return "", fmt.Errorf("locale '%s' not found", localeKey)
	}

	message, ok := locale.Messages[messageKey]
	if !ok {
		return "", fmt.Errorf("message key '%s' not found in locale '%s'", messageKey, localeKey)
	}

	return message, nil
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
