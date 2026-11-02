package translator

import (
	"bytes"
	"log/slog"
	"strings"
	"testing"

	"hue-bridge-emulator/internal/domain/model"
)

func capturingLogger() (*slog.Logger, *bytes.Buffer) {
	var buf bytes.Buffer
	return slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug})), &buf
}

func TestFactory_LogsUnknownType(t *testing.T) {
	l, buf := capturingLogger()
	f := NewFactory(l)
	f.Register(model.MappingTypeLight, NewLightStrategy(l))
	f.GetTranslator("nope")
	if !strings.Contains(buf.String(), "Unknown mapping type") || !strings.Contains(buf.String(), "nope") {
		t.Errorf("expected warning, got %q", buf.String())
	}
}

func TestStrategies_LogActionConfigOverrides(t *testing.T) {
	l, buf := capturingLogger()
	vd := &model.VirtualDevice{EntityID: "light.a", ActionConfig: &model.ActionConfig{OnService: "x.y", OnPayload: map[string]interface{}{"secret": "v"}}}
	for _, tr := range []Translator{NewLightStrategy(l), NewCoverStrategy(l), NewClimateStrategy(l), NewCustomStrategy(l)} {
		buf.Reset()
		tr.ToHA(&model.DeviceState{On: true}, vd)
		if !strings.Contains(buf.String(), "applying ActionConfig overrides") {
			t.Errorf("%T: no log, got %q", tr, buf.String())
		}
		if strings.Contains(buf.String(), "secret") {
			t.Errorf("%T: payload leaked in log", tr)
		}
	}
}
