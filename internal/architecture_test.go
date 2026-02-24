package internal

import (
	"github.com/kcmvp/archunit"
	"testing"
)

func TestArchitecture(t *testing.T) {
	domain := archunit.Packages("domain", []string{".../internal/domain/..."})
	adapters := archunit.Packages("adapters", []string{".../internal/adapters/..."})

	// Rule: Domain should not depend on adapters
	if err := domain.ShouldNotReferLayers(adapters); err != nil {
		t.Errorf("Architecture violation: %v", err)
	}
}
