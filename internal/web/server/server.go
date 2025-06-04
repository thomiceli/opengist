package server

import (
	"github.com/thomiceli/opengist/internal/validator"
	"net"
	"net/http"
	"os"

	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
	"github.com/thomiceli/opengist/internal/i18n"
)

type Server struct {
	echo *echo.Echo

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

func (s *Server) StartUnixSocket() {
	socketPath := "/tmp/opengist.sock"
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		log.Warn().Err(err).Str("socket", socketPath).Msg("Failed to remove existing socket file")
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start Unix socket server")
	}
	s.echo.Listener = listener

	log.Info().Msgf("Starting Unix socket server on " + socketPath)
	server := new(http.Server)
	if err := s.echo.StartServer(server); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Failed to start Unix socket server")
	}
}

func (s *Server) Stop() {
	if err := s.echo.Close(); err != nil {
		log.Fatal().Err(err).Msg("Failed to stop HTTP server")
	}
}

func (s *Server) StopUnixSocket() {
	log.Info().Msg("Stopping Unix socket server...")

	var socketPath string
	if s.echo.Listener != nil {
		if unixListener, ok := s.echo.Listener.(*net.UnixListener); ok {
			socketPath = unixListener.Addr().String()
		}
	}

	if err := s.echo.Close(); err != nil {
		log.Error().Err(err).Msg("Failed to stop Unix socket server")
	}

	if socketPath != "" {
		if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
			log.Error().Err(err).Str("socket", socketPath).Msg("Failed to remove socket file")
		} else {
			log.Info().Str("socket", socketPath).Msg("Socket file removed")
		}
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.echo.ServeHTTP(w, r)
}
