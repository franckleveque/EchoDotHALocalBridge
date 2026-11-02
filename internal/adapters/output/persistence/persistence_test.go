package persistence

import (
	"bytes"
	"context"
	"github.com/stretchr/testify/assert"
	"hue-bridge-emulator/internal/domain/model"
	"log/slog"
	"os"
	"path/filepath"
	"testing"
)

var testLogger = slog.New(slog.DiscardHandler)

func TestJSONConfigRepository_Migration(t *testing.T) {
	tmpFile := "test_config_legacy.json"
	defer os.Remove(tmpFile)

	legacyData := `{
		"hass_url": "http://ha:8123",
		"hass_token": "secret",
		"entity_mappings": {
			"light.living_room": {
				"entity_id": "light.living_room",
				"hue_id": "1",
				"name": "Living Room",
				"type": "light",
				"exposed": true,
				"custom_formula": {
					"to_hue_formula": "x * 2.54"
				}
			}
		}
	}`
	os.WriteFile(tmpFile, []byte(legacyData), 0644)

	repo := NewJSONConfigRepository(tmpFile, testLogger)
	cfg, err := repo.Get(context.Background())

	assert.NoError(t, err)
	assert.Equal(t, "http://ha:8123", cfg.HassURL)
	assert.Len(t, cfg.VirtualDevices, 1)
	assert.Equal(t, "1", cfg.VirtualDevices[0].HueID)
	assert.Equal(t, "x * 2.54", cfg.VirtualDevices[0].ActionConfig.ToHueFormula)
}

func TestJSONConfigRepository_NewFormat(t *testing.T) {
	tmpFile := "test_config_new.json"
	defer os.Remove(tmpFile)

	repo := NewJSONConfigRepository(tmpFile, testLogger)
	cfg := &model.Config{
		HassURL: "http://ha:8123",
		VirtualDevices: []*model.VirtualDevice{
			{HueID: "1", Name: "Test", EntityID: "light.test", Type: model.MappingTypeLight},
		},
	}

	err := repo.Save(context.Background(), cfg)
	assert.NoError(t, err)

	loaded, err := repo.Get(context.Background())
	assert.NoError(t, err)
	assert.Len(t, loaded.VirtualDevices, 1)
	assert.Equal(t, "Test", loaded.VirtualDevices[0].Name)
}

func TestSelectKeySize(t *testing.T) {
	cases := map[int]int{40: 32, 32: 32, 28: 24, 24: 24, 20: 16, 16: 16, 10: 0, 0: 0}
	for n, want := range cases {
		assert.Equal(t, want, selectKeySize(n), "n=%d", n)
	}
}

func TestNewJSONConfigRepository_KeyFromEnv(t *testing.T) {
	t.Setenv("HUE_ENCRYPTION_KEY", "0123456789abcdefghijklmnopq") // 27 chars -> 24
	assert.Equal(t, []byte("0123456789abcdefghijklmn"), NewJSONConfigRepository("x", testLogger).key)
	t.Setenv("HUE_ENCRYPTION_KEY", "short")
	r := NewJSONConfigRepository("x", testLogger)
	assert.Equal(t, r.defaultKey, r.key)
}

func TestJSONAuthRepository_SaveGetAndLogsWriteFailure(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	path := filepath.Join(t.TempDir(), "auth.json")
	repo := NewJSONAuthRepository(path, logger)
	assert.False(t, repo.Exists())
	assert.NoError(t, repo.Save(context.Background(), &model.AuthConfig{Username: "u", Password: "h"}))
	got, err := repo.Get(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "u", got.Username)

	bad := NewJSONAuthRepository(filepath.Join(t.TempDir(), "missing", "auth.json"), logger)
	assert.Error(t, bad.Save(context.Background(), &model.AuthConfig{}))
	_, err = bad.Get(context.Background())
	assert.Error(t, err)
	assert.Contains(t, buf.String(), "Auth: write failed")
	assert.Contains(t, buf.String(), "Auth: read failed")
}

func TestJSONConfigRepository_WarnsOnPlaintextTokenWithoutLeakingIt(t *testing.T) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, nil))
	path := filepath.Join(t.TempDir(), "c.json")
	os.WriteFile(path, []byte(`{"hass_url":"u","hass_token":"plain token!","virtual_devices":[{"hue_id":"1","name":"n","entity_id":"light.a","type":"light"}]}`), 0600)
	cfg, err := NewJSONConfigRepository(path, logger).Get(context.Background())
	assert.NoError(t, err)
	assert.Equal(t, "plain token!", cfg.HassToken)
	assert.Contains(t, buf.String(), "not encrypted")
	assert.NotContains(t, buf.String(), "plain token!")
}
