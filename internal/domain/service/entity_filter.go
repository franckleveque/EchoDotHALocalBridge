package service

import (
	"log/slog"
	"strings"
)

// EntityFilter decides which Home Assistant entities the bridge can expose.
type EntityFilter struct {
	ignoredDomains []string
	logger         *slog.Logger
}

func NewEntityFilter(ignoredDomains []string, logger *slog.Logger) *EntityFilter {
	return &EntityFilter{ignoredDomains: ignoredDomains, logger: logger}
}

// Supports reports whether the entity is exposable: not in an ignored domain
// and shaped like "domain.object_id".
func (f *EntityFilter) Supports(entityID string) bool {
	for _, domain := range f.ignoredDomains {
		if strings.HasPrefix(entityID, domain) {
			f.logger.Debug("Entity ignored: domain excluded", "entity_id", entityID, "domain", domain)
			return false
		}
	}
	supported := strings.Contains(entityID, ".")
	if !supported {
		f.logger.Debug("Entity ignored: malformed entity id", "entity_id", entityID)
	}
	return supported
}
