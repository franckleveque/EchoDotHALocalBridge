package translator

import (
	"hue-bridge-emulator/internal/domain/model"
	"log/slog"
)

type Factory struct {
	strategies map[model.MappingType]Translator
	logger     *slog.Logger
}

func NewFactory(logger *slog.Logger) *Factory {
	return &Factory{
		strategies: make(map[model.MappingType]Translator),
		logger:     logger,
	}
}

// Register binds a strategy to a mapping type.
func (f *Factory) Register(mappingType model.MappingType, strategy Translator) {
	f.strategies[mappingType] = strategy
}

// resolve returns the mapping type whose strategy handles mappingType. An
// unknown type logs a warning and falls back to the light strategy.
func (f *Factory) resolve(mappingType model.MappingType) model.MappingType {
	if _, ok := f.strategies[mappingType]; ok {
		return mappingType
	}
	f.logger.Warn("Unknown mapping type, falling back to light strategy", "type", mappingType)
	return model.MappingTypeLight
}

// GetTranslator returns the strategy for the given mapping type (light for unknown types).
func (f *Factory) GetTranslator(mappingType model.MappingType) Translator {
	return f.strategies[f.resolve(mappingType)]
}

func (f *Factory) ToHue(mappingType model.MappingType, haState model.HAEntityState, vd *model.VirtualDevice) *model.DeviceState {
	return f.strategies[f.resolve(mappingType)].ToHue(haState, vd)
}

func (f *Factory) ToHA(mappingType model.MappingType, hueState *model.DeviceState, vd *model.VirtualDevice) model.HomeAssistantCommand {
	return f.strategies[f.resolve(mappingType)].ToHA(hueState, vd)
}

func (f *Factory) Metadata(mappingType model.MappingType) model.HueMetadata {
	return f.strategies[f.resolve(mappingType)].GetMetadata()
}
