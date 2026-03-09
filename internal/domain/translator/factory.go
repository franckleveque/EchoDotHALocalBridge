package translator

import (
	"hue-bridge-emulator/internal/domain/model"
	"log/slog"
)

type Factory struct {
	strategies map[model.MappingType]Translator
}

func NewFactory() *Factory {
	return &Factory{
		strategies: map[model.MappingType]Translator{
			model.MappingTypeLight:   &LightStrategy{},
			model.MappingTypeCover:   &CoverStrategy{},
			model.MappingTypeClimate: &ClimateStrategy{},
			model.MappingTypeCustom:  &CustomStrategy{},
		},
	}
}

// GetTranslator returns the strategy for the given mapping type.
// If the type is unknown, it logs a warning and falls back to LightStrategy.
func (f *Factory) GetTranslator(mappingType model.MappingType) Translator {
	if t, ok := f.strategies[mappingType]; ok {
		return t
	}
	// Fallback to light strategy for unknown types
	slog.Warn("Unknown mapping type, falling back to light strategy", "type", mappingType)
	return f.strategies[model.MappingTypeLight]
}
