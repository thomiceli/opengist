package context

import (
	"net/http"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gorilla/sessions"
	"github.com/markbates/goth/gothic"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/session"
)

var gothicStoreOnce sync.Once

type Store struct {
	sessionsPath string

	flashStore *sessions.CookieStore
	UserStore  *sessions.FilesystemStore
}

func NewStore(sessionsPath string) *Store {
	s := &Store{sessionsPath: sessionsPath}

	s.flashStore = sessions.NewCookieStore([]byte("opengist"))
	hardenCookie(s.flashStore.Options)
	encryptKey, _ := session.GenerateSecretKey(filepath.Join(s.sessionsPath, "session-encrypt.key"))
	s.UserStore = sessions.NewFilesystemStore(s.sessionsPath, config.SecretKey, encryptKey)
	s.UserStore.MaxLength(10 * 1024)
	hardenCookie(s.UserStore.Options)
	gothicStoreOnce.Do(func() {
		gothic.Store = s.UserStore
	})

	return s
}

// hardenCookie applies secure defaults to a session cookie. HttpOnly keeps the
// cookie out of reach of JavaScript (so an XSS cannot read the auth session),
// and SameSite=Lax mitigates CSRF. Secure is set only when the configured
// external URL is HTTPS, otherwise the cookie would never be sent over a
// plain-HTTP deployment and logins would silently break. It is set both ways
// because sessions.NewCookieStore defaults Secure to true: left alone, the flash
// cookie is dropped by the browser on plain HTTP and no flash message is shown.
func hardenCookie(opts *sessions.Options) {
	if opts == nil {
		return
	}
	opts.HttpOnly = true
	opts.SameSite = http.SameSiteLaxMode
	opts.Secure = strings.HasPrefix(strings.ToLower(config.C.ExternalUrl), "https://")
}
