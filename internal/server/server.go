package server

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"

	"github.com/n3tuk/action-pull-request-deployment-lock/internal/config"
)

// Server manages the three HTTP servers (API, Probe, Metrics).
type Server struct {
	cfg           *config.Config
	logger        *zap.Logger
	apiServer     *http.Server
	probeServer   *http.Server
	metricsServer *http.Server
	startTime     time.Time
	shutdownChan  chan struct{}

	// Prometheus metrics
	appInfo       *prometheus.GaugeVec
	appUptime     prometheus.Counter
	httpRequests  *prometheus.CounterVec
	httpDuration  *prometheus.HistogramVec
	requestsInFlt prometheus.Gauge
}

// New creates a new Server instance.
func New(cfg *config.Config, logger *zap.Logger) (*Server, error) {
	s := &Server{
		cfg:          cfg,
		logger:       logger,
		startTime:    time.Now(),
		shutdownChan: make(chan struct{}),
	}

	// Initialize metrics
	s.initMetrics()

	// Setup servers
	if err := s.setupServers(); err != nil {
		return nil, err
	}

	return s, nil
}

// initMetrics initializes Prometheus metrics.
func (s *Server) initMetrics() {
	s.appInfo = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "app_info",
			Help: "Application build information",
		},
		[]string{"version"},
	)

	s.appUptime = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "app_uptime_seconds",
			Help: "Application uptime in seconds",
		},
	)

	s.httpRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests",
		},
		[]string{"server", "method", "path", "status"},
	)

	s.httpDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "HTTP request duration in seconds",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"server", "method", "path"},
	)

	s.requestsInFlt = prometheus.NewGauge(
		prometheus.GaugeOpts{
			Name: "http_requests_in_flight",
			Help: "Current number of HTTP requests being processed",
		},
	)

	// Try to register metrics, ignore if already registered (for tests)
	_ = prometheus.Register(s.appInfo)
	_ = prometheus.Register(s.appUptime)
	_ = prometheus.Register(s.httpRequests)
	_ = prometheus.Register(s.httpDuration)
	_ = prometheus.Register(s.requestsInFlt)

	// Set app info
	s.appInfo.WithLabelValues("dev").Set(1)
}

// setupServers configures the three HTTP servers.
func (s *Server) setupServers() error {
	// API Server
	apiRouter := s.setupAPIRouter()
	s.apiServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.cfg.APIHost, s.cfg.APIPort),
		Handler:      apiRouter,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	if s.cfg.TLSEnabled {
		tlsConfig := &tls.Config{
			MinVersion: tls.VersionTLS12,
		}
		s.apiServer.TLSConfig = tlsConfig
	}

	// Probe Server
	probeRouter := s.setupProbeRouter()
	s.probeServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.cfg.ProbeHost, s.cfg.ProbePort),
		Handler:      probeRouter,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 5 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	// Metrics Server
	metricsRouter := s.setupMetricsRouter()
	s.metricsServer = &http.Server{
		Addr:         fmt.Sprintf("%s:%d", s.cfg.MetricsHost, s.cfg.MetricsPort),
		Handler:      metricsRouter,
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
		IdleTimeout:  30 * time.Second,
	}

	return nil
}

// setupAPIRouter creates the API server router with middleware.
func (s *Server) setupAPIRouter() *chi.Mux {
	r := chi.NewRouter()

	// Middleware
	r.Use(middleware.RequestID)
	r.Use(s.loggingMiddleware("api"))
	r.Use(middleware.Recoverer)
	r.Use(s.metricsMiddleware("api"))

	// Routes
	setupAPIRoutes(r, s.logger)

	return r
}

// setupProbeRouter creates the probe server router.
func (s *Server) setupProbeRouter() *chi.Mux {
	r := chi.NewRouter()

	// Routes
	setupProbeRoutes(r, s.logger)

	return r
}

// setupMetricsRouter creates the metrics server router.
func (s *Server) setupMetricsRouter() *chi.Mux {
	r := chi.NewRouter()

	// Routes
	r.Handle("/metrics", promhttp.Handler())

	return r
}

// loggingMiddleware logs HTTP requests.
func (s *Server) loggingMiddleware(serverName string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			s.logger.Info("HTTP request",
				zap.String("server", serverName),
				zap.String("method", r.Method),
				zap.String("path", r.URL.Path),
				zap.Int("status", ww.Status()),
				zap.Duration("duration", time.Since(start)),
				zap.String("request_id", middleware.GetReqID(r.Context())),
			)
		})
	}
}

// metricsMiddleware records HTTP metrics.
func (s *Server) metricsMiddleware(serverName string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			s.requestsInFlt.Inc()
			defer s.requestsInFlt.Dec()

			ww := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
			next.ServeHTTP(ww, r)

			duration := time.Since(start).Seconds()
			status := fmt.Sprintf("%d", ww.Status())

			s.httpRequests.WithLabelValues(serverName, r.Method, r.URL.Path, status).Inc()
			s.httpDuration.WithLabelValues(serverName, r.Method, r.URL.Path).Observe(duration)
		})
	}
}

// Start starts all three HTTP servers.
func (s *Server) Start() error {
	var wg sync.WaitGroup
	errChan := make(chan error, 3)

	// Start API server
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.logger.Info("Starting API server", zap.String("addr", s.apiServer.Addr))

		var err error
		if s.cfg.TLSEnabled {
			err = s.apiServer.ListenAndServeTLS(s.cfg.TLSCert, s.cfg.TLSKey)
		} else {
			err = s.apiServer.ListenAndServe()
		}

		if err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("API server error: %w", err)
		}
	}()

	// Start probe server
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.logger.Info("Starting probe server", zap.String("addr", s.probeServer.Addr))

		if err := s.probeServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("probe server error: %w", err)
		}
	}()

	// Start metrics server
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.logger.Info("Starting metrics server", zap.String("addr", s.metricsServer.Addr))

		if err := s.metricsServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- fmt.Errorf("metrics server error: %w", err)
		}
	}()

	// Wait a bit to see if any server fails to start
	time.Sleep(100 * time.Millisecond)
	select {
	case err := <-errChan:
		return err
	default:
		// Start uptime counter goroutine
		go s.updateUptime()
		return nil
	}
}

// updateUptime updates the uptime metric periodically.
func (s *Server) updateUptime() {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.appUptime.Add(1)
		case <-s.shutdownChan:
			return
		}
	}
}

// Shutdown gracefully shuts down all servers.
func (s *Server) Shutdown(ctx context.Context) error {
	s.logger.Info("Shutting down servers gracefully")

	// Signal the uptime goroutine to stop
	close(s.shutdownChan)

	var wg sync.WaitGroup
	errChan := make(chan error, 3)

	// Shutdown API server first
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.logger.Info("Shutting down API server")
		if err := s.apiServer.Shutdown(ctx); err != nil {
			errChan <- fmt.Errorf("API server shutdown error: %w", err)
		}
	}()

	// Shutdown metrics server second
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.logger.Info("Shutting down metrics server")
		if err := s.metricsServer.Shutdown(ctx); err != nil {
			errChan <- fmt.Errorf("metrics server shutdown error: %w", err)
		}
	}()

	// Shutdown probe server last
	wg.Add(1)
	go func() {
		defer wg.Done()
		s.logger.Info("Shutting down probe server")
		if err := s.probeServer.Shutdown(ctx); err != nil {
			errChan <- fmt.Errorf("probe server shutdown error: %w", err)
		}
	}()

	wg.Wait()
	close(errChan)

	for err := range errChan {
		if err != nil {
			return err
		}
	}

	s.logger.Info("All servers shut down successfully")
	return nil
}

// WaitForServers waits for all servers to be ready.
func (s *Server) WaitForServers(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		if s.checkServer(s.apiServer.Addr) &&
			s.checkServer(s.probeServer.Addr) &&
			s.checkServer(s.metricsServer.Addr) {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	return fmt.Errorf("servers did not become ready within %s", timeout)
}

// checkServer checks if a server is listening on the given address.
func (s *Server) checkServer(addr string) bool {
	conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
	if err != nil {
		return false
	}
	conn.Close()
	return true
}
