package internal

import (
	"github.com/kcmvp/archunit"
	"testing"
)

func TestArchitecture(t *testing.T) {
	domain := archunit.Packages("domain", []string{".../internal/domain/..."})
	adapters := archunit.Packages("adapters", []string{".../internal/adapters/..."})

	// Rule 1: Domain should not depend on adapters
	if err := domain.ShouldNotReferLayers(adapters); err != nil {
		t.Errorf("Architecture violation: Domain depends on Adapters: %v", err)
	}
}

func TestSOLID(t *testing.T) {
	// Simple check for translator package presence
	translator := archunit.Packages("translator", []string{".../internal/domain/translator"})
	if len(translator.Packages()) == 0 {
		t.Error("No translator package found in domain")
	}
}
