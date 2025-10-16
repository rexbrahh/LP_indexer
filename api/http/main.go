package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"

	"github.com/rexbrahh/lp-indexer/api/http/cache"
	apitypes "github.com/rexbrahh/lp-indexer/api/http/types"
)

// Server bundles dependencies for the HTTP API.
type Server struct {
	router  *chi.Mux
	cache   *cache.Cache
	logger  *log.Logger
	started time.Time
}

// NewServer constructs a Server with registered routes.
func NewServer(cacheClient *cache.Cache, logger *log.Logger) *Server {
	if logger == nil {
		logger = log.New(os.Stdout, "api-http ", log.LstdFlags|log.Lshortfile)
	}

	s := &Server{
		router:  chi.NewRouter(),
		cache:   cacheClient,
		logger:  logger,
		started: time.Now(),
	}

	s.router.Get("/healthz", s.healthzHandler)
	s.router.Route("/v1", func(r chi.Router) {
		r.Get("/pool/{id}", s.poolHandler)
		r.Get("/pool/{id}/candles", s.candlesHandler)
	})

	return s
}

// Handler exposes the underlying router for integration tests.
func (s *Server) Handler() http.Handler {
	return s.router
}

func (s *Server) healthzHandler(w http.ResponseWriter, r *http.Request) {
	resp := apitypes.HealthResponse{
		Status: "ok",
		Uptime: time.Since(s.started).Round(time.Millisecond).String(),
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) poolHandler(w http.ResponseWriter, r *http.Request) {
	poolID := chi.URLParam(r, "id")
	resp := apitypes.PoolResponse{
		ID:         poolID,
		Name:       "stub-pool",
		BaseAsset:  "SOL",
		QuoteAsset: "USDC",
	}

	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) candlesHandler(w http.ResponseWriter, r *http.Request) {
	poolID := chi.URLParam(r, "id")
	timeframe := r.URL.Query().Get("tf")

	if err := apitypes.ValidateTimeframe(timeframe); err != nil {
		writeJSON(w, http.StatusBadRequest, apitypes.ErrorResponse{Error: err.Error()})
		return
	}

	ctx := r.Context()
	var (
		candles []apitypes.Candle
		err     error
	)

	if s.cache != nil {
		candles, err = s.cache.GetCandles(ctx, poolID, timeframe)
	}

	if err != nil {
		if errors.Is(err, cache.ErrDisabled) || errors.Is(err, apitypes.ErrNotFound) {
			candles = stubCandles(timeframe)
		} else {
			s.logger.Printf("cache get failed: %v", err)
			writeJSON(w, http.StatusInternalServerError, apitypes.ErrorResponse{Error: "internal error"})
			return
		}
	}

	if candles == nil {
		candles = stubCandles(timeframe)
	}

	resp := apitypes.CandlesResponse{
		PoolID:    poolID,
		Timeframe: timeframe,
		Candles:   candles,
	}

	writeJSON(w, http.StatusOK, resp)

	// Seed the cache with stub data when available.
	if s.cache != nil {
		go func() {
			if err := s.cache.SetCandles(context.Background(), poolID, timeframe, candles); err != nil && !errors.Is(err, cache.ErrDisabled) {
				s.logger.Printf("cache set failed: %v", err)
			}
		}()
	}
}

func stubCandles(tf string) []apitypes.Candle {
	base := time.Unix(0, 0).UTC()
	step := durationForTimeframe(tf)
	return []apitypes.Candle{
		{
			Timestamp: base,
			Open:      25.10,
			High:      25.42,
			Low:       24.98,
			Close:     25.30,
			Volume:    1_250,
		},
		{
			Timestamp: base.Add(step),
			Open:      25.30,
			High:      25.60,
			Low:       25.12,
			Close:     25.50,
			Volume:    1_480,
		},
	}
}

func durationForTimeframe(tf string) time.Duration {
	switch tf {
	case "1m":
		return time.Minute
	case "5m":
		return 5 * time.Minute
	case "1h":
		return time.Hour
	case "1d":
		return 24 * time.Hour
	default:
		return time.Minute
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func main() {
	logger := log.New(os.Stdout, "api-http ", log.LstdFlags|log.Lshortfile)

	cfg, err := cache.LoadConfigFromEnv()
	if err != nil {
		logger.Fatalf("load redis config: %v", err)
	}

	cacheClient, err := cache.New(cfg)
	if err != nil {
		logger.Fatalf("init redis cache: %v", err)
	}
	if !cfg.Enabled {
		logger.Println("redis cache disabled: API_REDIS_ADDR not set")
	}

	server := NewServer(cacheClient, logger)

	addr := os.Getenv("API_HTTP_ADDR")
	if addr == "" {
		addr = ":8080"
	}

	srv := &http.Server{
		Addr:    addr,
		Handler: server.Handler(),
	}

	go func() {
		logger.Printf("HTTP server listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Fatalf("listen: %v", err)
		}
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	<-sigCh
	logger.Println("shutdown signal received")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctx); err != nil {
		logger.Printf("graceful shutdown failed: %v", err)
	}
}
