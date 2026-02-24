package translator

import (
	"hue-bridge-emulator/internal/domain/model"
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

func (f *Factory) GetTranslator(mappingType model.MappingType) Translator {
	if t, ok := f.strategies[mappingType]; ok {
		return t
	}
	return f.strategies[model.MappingTypeLight]
}
