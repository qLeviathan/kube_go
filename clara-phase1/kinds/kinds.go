// Package kinds defines the AR kind taxonomy for CARLA.
// Pure rules engine — no ML kinds.
package kinds

import "fmt"

// Kind identifies a specific AR technique family.
type Kind struct {
	ID       string
	Name     string
	Parent   string // broader kind ID, empty if top-level
}

// Validate checks that the kind has valid fields.
func (k Kind) Validate() error {
	if k.ID == "" {
		return fmt.Errorf("kind: empty ID")
	}
	return nil
}

// AR kinds (Logic Programs as the core reasoning engine).
var (
	KindLogicPrograms = Kind{ID: "ar-lp", Name: "Logic Programs"}
	KindBayesianLP    = Kind{ID: "ar-blp", Name: "Bayesian Logic Programs", Parent: "ar-lp"}
	KindClassicalProp = Kind{ID: "ar-prop", Name: "Propositional Classical Logic"}
)

// Registry returns all registered kinds.
func Registry() []Kind {
	return []Kind{
		KindLogicPrograms, KindBayesianLP, KindClassicalProp,
	}
}

// ARKinds returns all AR kinds (same as Registry since we only have AR).
func ARKinds() []Kind {
	return Registry()
}
