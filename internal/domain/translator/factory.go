package translator

import (
	"hue-bridge-emulator/internal/domain/model"
)

type Factory struct {
	strategies map[model.DeviceType]Translator
}

func NewFactory() *Factory {
	return &Factory{
		strategies: map[model.DeviceType]Translator{
			model.DeviceTypeLight:   &LightStrategy{},
			model.DeviceTypeCover:   &CoverStrategy{},
			model.DeviceTypeClimate: &ClimateStrategy{},
		},
	}
}

func (f *Factory) GetTranslator(deviceType model.DeviceType) Translator {
	if t, ok := f.strategies[deviceType]; ok {
		return t
	}
	return f.strategies[model.DeviceTypeLight]
}
