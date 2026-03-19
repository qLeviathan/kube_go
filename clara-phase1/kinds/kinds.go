// Package kinds defines the ML and AR kind taxonomy for CLARA Phase 1.
// Per DARPA-PA-25-07-02 Section I.H: Phase 1 requires >=1 ML kind and >=1 AR kind.
package kinds

import "fmt"

// Category is either ML or AR.
type Category string

const (
	CategoryML Category = "ML"
	CategoryAR Category = "AR"
)

// ValidCategory returns true if c is a known category.
func ValidCategory(c Category) bool {
	return c == CategoryML || c == CategoryAR
}

// Kind identifies a specific ML or AR technique family.
type Kind struct {
	ID       string
	Name     string
	Category Category
	Parent   string // broader kind ID, empty if top-level
}

// Validate checks that the kind has valid fields.
func (k Kind) Validate() error {
	if k.ID == "" {
		return fmt.Errorf("kind: empty ID")
	}
	if !ValidCategory(k.Category) {
		return fmt.Errorf("kind %q: invalid category %q", k.ID, k.Category)
	}
	return nil
}

// Phase 1 AR kinds (Logic Programs selected as AR departure point).
var (
	KindLogicPrograms = Kind{ID: "ar-lp", Name: "Logic Programs", Category: CategoryAR}
	KindBayesianLP    = Kind{ID: "ar-blp", Name: "Bayesian Logic Programs", Category: CategoryAR, Parent: "ar-lp"}
	KindClassicalProp = Kind{ID: "ar-prop", Name: "Propositional Classical Logic", Category: CategoryAR}
)

// Phase 1 ML kinds (Bayesian selected as ML departure point).
var (
	KindBayesian     = Kind{ID: "ml-bayes", Name: "Bayesian", Category: CategoryML}
	KindBayesNets    = Kind{ID: "ml-bn", Name: "Bayesian Networks", Category: CategoryML, Parent: "ml-bayes"}
	KindDecisionTree = Kind{ID: "ml-dt", Name: "Decision Trees", Category: CategoryML, Parent: "ml-bayes"}
)

// Registry returns all registered kinds for Phase 1.
func Registry() []Kind {
	return []Kind{
		KindLogicPrograms, KindBayesianLP, KindClassicalProp,
		KindBayesian, KindBayesNets, KindDecisionTree,
	}
}

// MLKinds returns only ML kinds.
func MLKinds() []Kind {
	var out []Kind
	for _, k := range Registry() {
		if k.Category == CategoryML {
			out = append(out, k)
		}
	}
	return out
}

// ARKinds returns only AR kinds.
func ARKinds() []Kind {
	var out []Kind
	for _, k := range Registry() {
		if k.Category == CategoryAR {
			out = append(out, k)
		}
	}
	return out
}
