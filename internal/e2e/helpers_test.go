//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	httpAdapter "hue-bridge-emulator/internal/adapters/input/http"
	"hue-bridge-emulator/internal/adapters/output/homeassistant"
	"hue-bridge-emulator/internal/adapters/output/logging"
	"hue-bridge-emulator/internal/adapters/output/persistence"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/domain/service"
	"hue-bridge-emulator/internal/domain/translator"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
)

// nextClientIP numbers the synthetic client addresses handed to test requests.
var nextClientIP atomic.Uint32

// fakeHA simulates the Home Assistant REST API.
type fakeHA struct {
	mu     sync.Mutex
	states []map[string]interface{} // returned by GET /api/states
	calls  []haServiceCall          // recorded by POST /api/services/...
	// failStatus/failBody, when set, make service calls answer with an error.
	failStatus int
	failBody   string
	server     *httptest.Server
}

type haServiceCall struct {
	Domain  string
	Service string
	Payload map[string]interface{}
}

func newFakeHA(t *testing.T, states []map[string]interface{}) *fakeHA {
	t.Helper()
	f := &fakeHA{states: states}
	mux := http.NewServeMux()

	mux.HandleFunc("/api/states", func(w http.ResponseWriter, r *http.Request) {
		fmt.Println("fakeHA: GET /api/states")
		f.mu.Lock()
		defer f.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(f.states)
	})

	mux.HandleFunc("/api/services/", func(w http.ResponseWriter, r *http.Request) {
		// path: /api/services/{domain}/{service}
		parts := strings.Split(strings.TrimPrefix(r.URL.Path, "/api/services/"), "/")
		if len(parts) < 2 {
			http.Error(w, "Bad Request", http.StatusBadRequest)
			return
		}
		var payload map[string]interface{}
		json.NewDecoder(r.Body).Decode(&payload)
		f.mu.Lock()
		f.calls = append(f.calls, haServiceCall{
			Domain: parts[0], Service: parts[1], Payload: payload,
		})
		failStatus, failBody := f.failStatus, f.failBody
		f.mu.Unlock()
		if failStatus != 0 {
			w.WriteHeader(failStatus)
			w.Write([]byte(failBody))
			return
		}
		w.WriteHeader(http.StatusOK)
	})

	f.server = httptest.NewServer(mux)
	t.Cleanup(f.server.Close)
	return f
}

func (f *fakeHA) lastCall() haServiceCall {
	f.mu.Lock()
	defer f.mu.Unlock()
	if len(f.calls) == 0 {
		return haServiceCall{}
	}
	return f.calls[len(f.calls)-1]
}

func (f *fakeHA) callCount() int {
	f.mu.Lock()
	defer f.mu.Unlock()
	return len(f.calls)
}

// testAuthRepo is a simple in-memory auth repository for testing.
type testAuthRepo struct {
	exists bool
	auth   *model.AuthConfig
}

func (r *testAuthRepo) Exists() bool {
	return r.exists
}

func (r *testAuthRepo) Save(ctx context.Context, auth *model.AuthConfig) error {
	r.auth = auth
	r.exists = true
	return nil
}

func (r *testAuthRepo) Get(ctx context.Context) (*model.AuthConfig, error) {
	return r.auth, nil
}

func newTestStack(t *testing.T, ha *fakeHA, cfg *model.Config) *httptest.Server {
	t.Helper()

	ring := logging.NewLineRing(500, 4096, slog.New(slog.DiscardHandler))
	logger := slog.New(logging.NewHandler(io.Discard, ring, slog.LevelInfo))

	// Real persistence on a temp file
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	cfgRepo := persistence.NewJSONConfigRepository(cfgPath, slog.New(slog.DiscardHandler))
	if cfg != nil {
		fmt.Printf("newTestStack: Saving initial config to %s\n", cfgPath)
		if err := cfgRepo.Save(context.Background(), cfg); err != nil {
			t.Fatalf("failed to save initial config: %v", err)
		}
	}

	authRepo := &testAuthRepo{}
	authService := service.NewAuthService(authRepo, logger)

	// Real HA client pointed at fake HA
	haClient := homeassistant.NewClient(logger)
	if ha != nil {
		haClient.Configure(ha.server.URL, "test-token")
	}

	translatorFactory := translator.NewFactory(logger)
	translatorFactory.Register(model.MappingTypeLight, translator.NewLightStrategy(logger))
	translatorFactory.Register(model.MappingTypeCover, translator.NewCoverStrategy(logger))
	translatorFactory.Register(model.MappingTypeClimate, translator.NewClimateStrategy(logger))
	translatorFactory.Register(model.MappingTypeCustom, translator.NewCustomStrategy(logger))

	bridgeSvc := service.NewBridgeService(haClient, cfgRepo, translatorFactory, logger)

	srv := httpAdapter.NewServer(bridgeSvc, bridgeSvc, authService, "127.0.0.1", logger)

	srv.SetLogSource(ring)
	mux := srv.Mux()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bypass the per-IP rate limiter: every request gets its own address.
		n := nextClientIP.Add(1)
		r.RemoteAddr = fmt.Sprintf("10.%d.%d.%d:1234", (n>>16)&0xff, (n>>8)&0xff, n&0xff)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts
}
