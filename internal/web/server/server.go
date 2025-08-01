package server

import (
	"fmt"
	"github.com/thomiceli/opengist/internal/validator"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

func isSocketPath(host string) bool {
	return strings.Contains(host, "/") || strings.Contains(host, "\\")
}

func (s *Server) Start() {
	if isSocketPath(config.C.HttpHost) {
		s.startUnixSocket()
	} else {
		s.startHTTP()
	}
}

func (s *Server) startHTTP() {
	addr := config.C.HttpHost + ":" + config.C.HttpPort

	log.Info().Msg("Starting HTTP server on http://" + addr)
	if err := s.echo.Start(addr); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Failed to start HTTP server")
	}
}

func (s *Server) startUnixSocket() {
	socketPath := config.C.HttpHost
	if socketPath == "" {
		socketPath = "/tmp/opengist.sock"
	}

	if dir := filepath.Dir(socketPath); dir != "." {
		if err := os.MkdirAll(dir, 0755); err != nil {
			log.Warn().Err(err).Str("dir", dir).Msg("Failed to create socket directory")
		}
	}
	if err := os.Remove(socketPath); err != nil && !os.IsNotExist(err) {
		log.Warn().Err(err).Str("socket", socketPath).Msg("Failed to remove existing socket file")
	}

	pidPath := strings.TrimSuffix(socketPath, filepath.Ext(socketPath)) + ".pid"
	if err := s.createPidFile(pidPath); err != nil {
		log.Warn().Err(err).Str("pid-file", pidPath).Msg("Failed to create PID file")
	}

	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		log.Fatal().Err(err).Msg("Failed to start Unix socket server")
	}
	s.echo.Listener = listener

	if config.C.UnixSocketPermissions != "" {
		if perm, err := strconv.ParseUint(config.C.UnixSocketPermissions, 8, 32); err == nil {
			if err := os.Chmod(socketPath, os.FileMode(perm)); err != nil {
				log.Warn().Err(err).Str("socket", socketPath).Str("permissions", config.C.UnixSocketPermissions).Msg("Failed to set socket permissions")
			}
		} else {
			log.Warn().Err(err).Str("permissions", config.C.UnixSocketPermissions).Msg("Invalid socket permissions format")
		}
	}

	log.Info().Str("socket", socketPath).Msg("Starting Unix socket server")
	log.Info().Str("pid-file", pidPath).Msg("PID file created")
	server := new(http.Server)
	if err := s.echo.StartServer(server); err != nil && err != http.ErrServerClosed {
		log.Fatal().Err(err).Msg("Failed to start Unix socket server")
	}
}

func (s *Server) Stop() {
	if isSocketPath(config.C.HttpHost) {
		s.stopUnixSocket()
	} else {
		s.stopHTTP()
	}
}

func (s *Server) stopHTTP() {
	log.Info().Msg("Stopping HTTP server...")
	if err := s.echo.Close(); err != nil {
		log.Fatal().Err(err).Msg("Failed to stop HTTP server")
	}
}

func (s *Server) stopUnixSocket() {
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

		pidPath := strings.TrimSuffix(socketPath, filepath.Ext(socketPath)) + ".pid"
		if err := os.Remove(pidPath); err != nil && !os.IsNotExist(err) {
			log.Error().Err(err).Str("pid-file", pidPath).Msg("Failed to remove PID file")
		} else {
			log.Info().Str("pid-file", pidPath).Msg("PID file removed")
		}
	}
}

func (s *Server) createPidFile(pidPath string) error {
	pid := os.Getpid()
	pidContent := fmt.Sprintf("%d\n", pid)

	if err := os.WriteFile(pidPath, []byte(pidContent), 0644); err != nil {
		return err
	}

	return nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.echo.ServeHTTP(w, r)
}
