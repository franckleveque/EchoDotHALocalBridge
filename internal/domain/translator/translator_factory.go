package translator

import (
	"hue-bridge-emulator/internal/domain/model"
)

type TranslatorFactory interface {
	GetTranslator(mappingType model.MappingType) Translator
}
