package main

import (
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/rexbrahh/lp-indexer/api/http/cache"
	apitypes "github.com/rexbrahh/lp-indexer/api/http/types"
)

func newTestServer(t *testing.T) *Server {
	t.Helper()
	cacheClient, err := cache.New(cache.Config{Enabled: false, TTL: time.Minute})
	if err != nil {
		t.Fatalf("failed to create cache: %v", err)
	}

	srv := NewServer(cacheClient, logDiscard())
	return srv
}

func logDiscard() *log.Logger {
	return log.New(io.Discard, "", 0)
}

func TestHealthzHandler(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp apitypes.HealthResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.Status != "ok" {
		t.Fatalf("unexpected status: %+v", resp)
	}
}

func TestPoolHandler(t *testing.T) {
	srv := newTestServer(t)

	req := httptest.NewRequest(http.MethodGet, "/v1/pool/ABCD", nil)
	rr := httptest.NewRecorder()

	srv.Handler().ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rr.Code)
	}

	var resp apitypes.PoolResponse
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("decode response: %v", err)
	}

	if resp.ID != "ABCD" {
		t.Fatalf("expected pool id ABCD, got %s", resp.ID)
	}
}

func TestCandlesHandler(t *testing.T) {
	srv := newTestServer(t)

	t.Run("valid timeframe", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/pool/ABCD/candles?tf=5m", nil)
		rr := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Fatalf("expected status 200, got %d", rr.Code)
		}

		var resp apitypes.CandlesResponse
		if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
			t.Fatalf("decode response: %v", err)
		}

		if resp.Timeframe != "5m" {
			t.Fatalf("expected timeframe 5m, got %s", resp.Timeframe)
		}

		if len(resp.Candles) == 0 {
			t.Fatalf("expected stub candles, got %#v", resp)
		}
	})

	t.Run("missing timeframe", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/pool/ABCD/candles", nil)
		rr := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rr.Code)
		}
	})

	t.Run("invalid timeframe", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/v1/pool/ABCD/candles?tf=13m", nil)
		rr := httptest.NewRecorder()

		srv.Handler().ServeHTTP(rr, req)

		if rr.Code != http.StatusBadRequest {
			t.Fatalf("expected status 400, got %d", rr.Code)
		}
	})
}
