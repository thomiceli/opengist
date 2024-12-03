package server

import (
	"github.com/thomiceli/opengist/internal/validator"
	"net/http"

	"github.com/gorilla/sessions"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/i18n"
)

type Server struct {
	echo       *echo.Echo
	flashStore *sessions.CookieStore     // session store for flash messages
	UserStore  *sessions.FilesystemStore // session store for user sessions

	dev          bool
	sessionsPath string
	ignoreCsrf   bool
}

func NewServer(isDev bool, sessionsPath string, ignoreCsrf bool) *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true
	e.Validator = validator.NewValidator()

	s := &Server{echo: e, dev: isDev, sessionsPath: sessionsPath, ignoreCsrf: ignoreCsrf}

	s.useCustomContext()

	if err := i18n.Locales.LoadAll(); err != nil {
		log.Fatal().Err(err).Msg("Failed to load locales")
	}

	s.registerMiddlewares()
	s.setFuncMap()
	s.echo.HTTPErrorHandler = s.errorHandler

	if !s.dev {
		s.parseManifestEntries()
	}

	s.registerRoutes()

	return s
}

func (s *Server) Start() {
	addr := config.C.HttpHost + ":" + config.C.HttpPort

	log.Info().Msg("Starting HTTP server on http://" + addr)
	if err := s.echo.Start(addr); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Failed to start HTTP server")
	}
}

func (s *Server) Stop() {
	if err := s.echo.Close(); err != nil {
		log.Fatal().Err(err).Msg("Failed to stop HTTP server")
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.echo.ServeHTTP(w, r)
}
