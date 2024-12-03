package context

import (
	"github.com/gorilla/sessions"
	"github.com/markbates/goth/gothic"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/utils"
	"path/filepath"
)

type Store struct {
	sessionsPath string

	flashStore *sessions.CookieStore
	UserStore  *sessions.FilesystemStore
}

func NewStore(sessionsPath string) *Store {
	return &Store{sessionsPath: sessionsPath}
}

func (s *Store) setupSessionStore() {
	s.flashStore = sessions.NewCookieStore([]byte("opengist"))
	encryptKey, _ := utils.GenerateSecretKey(filepath.Join(s.sessionsPath, "session-encrypt.key"))
	s.UserStore = sessions.NewFilesystemStore(s.sessionsPath, config.SecretKey, encryptKey)
	s.UserStore.MaxLength(10 * 1024)
	gothic.Store = s.UserStore
}
