//go:build e2e

package e2e_test

import (
	"context"
	"encoding/json"
	"fmt"
	httpAdapter "hue-bridge-emulator/internal/adapters/input/http"
	"hue-bridge-emulator/internal/adapters/output/homeassistant"
	"hue-bridge-emulator/internal/adapters/output/persistence"
	"hue-bridge-emulator/internal/domain/model"
	"hue-bridge-emulator/internal/domain/service"
	"hue-bridge-emulator/internal/domain/translator"
	"math/rand"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync"
	"testing"
)

// fakeHA simulates the Home Assistant REST API.
type fakeHA struct {
	mu     sync.Mutex
	states []map[string]interface{} // returned by GET /api/states
	calls  []haServiceCall          // recorded by POST /api/services/...
	server *httptest.Server
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
		f.mu.Unlock()
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

	// Real persistence on a temp file
	tmpDir := t.TempDir()
	cfgPath := filepath.Join(tmpDir, "config.json")
	cfgRepo := persistence.NewJSONConfigRepository(cfgPath)
	if cfg != nil {
		fmt.Printf("newTestStack: Saving initial config to %s\n", cfgPath)
		if err := cfgRepo.Save(context.Background(), cfg); err != nil {
			t.Fatalf("failed to save initial config: %v", err)
		}
	}

	authRepo := &testAuthRepo{}
	authService := service.NewAuthService(authRepo)

	// Real HA client pointed at fake HA
	haClient := homeassistant.NewClient()
	if ha != nil {
		haClient.Configure(ha.server.URL, "test-token")
	}

	translatorFactory := translator.NewFactory()
	translatorFactory.Register(model.MappingTypeLight, &translator.LightStrategy{})
	translatorFactory.Register(model.MappingTypeCover, &translator.CoverStrategy{})
	translatorFactory.Register(model.MappingTypeClimate, &translator.ClimateStrategy{})
	translatorFactory.Register(model.MappingTypeCustom, &translator.CustomStrategy{})

	bridgeSvc := service.NewBridgeService(haClient, cfgRepo, translatorFactory)

	srv := httpAdapter.NewServer(bridgeSvc, bridgeSvc, authService, "127.0.0.1")
	mux := srv.Mux()
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Bypass rate limiter by using a random RemoteAddr
		r.RemoteAddr = fmt.Sprintf("127.0.0.%d:1234", rand.Intn(254)+1)
		mux.ServeHTTP(w, r)
	}))
	t.Cleanup(ts.Close)
	return ts
}
