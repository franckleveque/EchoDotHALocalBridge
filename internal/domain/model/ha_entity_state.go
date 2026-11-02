package model

import "strings"

type HAEntityState struct {
	EntityID   string
	State      string
	Attributes HAFields
}

func (s HAEntityState) IsSupported(ignoredDomains []string) bool {
	for _, domain := range ignoredDomains {
		if strings.HasPrefix(s.EntityID, domain) {
			return false
		}
	}
	return strings.Contains(s.EntityID, ".")
}
