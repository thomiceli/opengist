package metrics

import (
	"net/http"

	"github.com/labstack/echo-contrib/echoprometheus"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog/log"
	"github.com/thomiceli/opengist/internal/config"
)

type Server struct {
	echo *echo.Echo
}

func NewServer() *Server {
	e := echo.New()
	e.HideBanner = true
	e.HidePort = true

	s := &Server{echo: e}

	initMetrics()

	e.GET("/metrics", func(ctx echo.Context) error {
		updateMetrics()
		return echoprometheus.NewHandler()(ctx)
	})

	return s
}

func (s *Server) Start() {
	addr := config.C.MetricsHost + ":" + config.C.MetricsPort
	log.Info().Msg("Starting metrics server on http://" + addr)
	if err := s.echo.Start(addr); err != nil && err != http.ErrServerClosed {
		log.Error().Err(err).Msg("Failed to start metrics server")
	}
}

func (s *Server) Stop() {
	log.Info().Msg("Stopping metrics server...")
	if err := s.echo.Close(); err != nil {
		log.Error().Err(err).Msg("Failed to stop metrics server")
	}
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.echo.ServeHTTP(w, r)
}
