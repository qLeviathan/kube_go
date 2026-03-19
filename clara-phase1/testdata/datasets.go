// Package testdata provides dummy test datasets for CLARA Phase 1 evaluation.
// Domain: Medical treatment decisions (multi-condition medical guidance).
// This is one of the DARPA-suggested application task domains.
package testdata

import "github.com/clara-phase1/kinds"

// MedicalTrainSet returns a training dataset for medical treatment decisions.
func MedicalTrainSet() kinds.DataSet {
	return kinds.DataSet{
		Name:  "medical-treatment-train",
		Split: "train",
		Items: []kinds.Datum{
			{Features: map[string]float64{"blood_pressure": 0.8, "glucose": 0.7, "heart_rate": 0.6}, Label: "treat_A", Score: 0.9},
			{Features: map[string]float64{"blood_pressure": 0.3, "glucose": 0.9, "heart_rate": 0.4}, Label: "treat_B", Score: 0.85},
			{Features: map[string]float64{"blood_pressure": 0.6, "glucose": 0.3, "heart_rate": 0.8}, Label: "treat_A", Score: 0.75},
			{Features: map[string]float64{"blood_pressure": 0.2, "glucose": 0.2, "heart_rate": 0.3}, Label: "no_treat", Score: 0.95},
			{Features: map[string]float64{"blood_pressure": 0.9, "glucose": 0.8, "heart_rate": 0.9}, Label: "treat_C", Score: 0.7},
			{Features: map[string]float64{"blood_pressure": 0.5, "glucose": 0.5, "heart_rate": 0.5}, Label: "treat_B", Score: 0.6},
			{Features: map[string]float64{"blood_pressure": 0.7, "glucose": 0.1, "heart_rate": 0.7}, Label: "treat_A", Score: 0.8},
			{Features: map[string]float64{"blood_pressure": 0.1, "glucose": 0.8, "heart_rate": 0.2}, Label: "treat_B", Score: 0.88},
			{Features: map[string]float64{"blood_pressure": 0.4, "glucose": 0.4, "heart_rate": 0.9}, Label: "treat_A", Score: 0.72},
			{Features: map[string]float64{"blood_pressure": 0.1, "glucose": 0.1, "heart_rate": 0.1}, Label: "no_treat", Score: 0.98},
		},
	}
}

// MedicalTestSet returns a test dataset for evaluation.
func MedicalTestSet() kinds.DataSet {
	return kinds.DataSet{
		Name:  "medical-treatment-test",
		Split: "test",
		Items: []kinds.Datum{
			{Features: map[string]float64{"blood_pressure": 0.85, "glucose": 0.75, "heart_rate": 0.65}, Label: "treat_A", Score: 0.0},
			{Features: map[string]float64{"blood_pressure": 0.25, "glucose": 0.85, "heart_rate": 0.35}, Label: "treat_B", Score: 0.0},
			{Features: map[string]float64{"blood_pressure": 0.15, "glucose": 0.15, "heart_rate": 0.2}, Label: "no_treat", Score: 0.0},
			{Features: map[string]float64{"blood_pressure": 0.9, "glucose": 0.9, "heart_rate": 0.85}, Label: "treat_C", Score: 0.0},
			{Features: map[string]float64{"blood_pressure": 0.55, "glucose": 0.45, "heart_rate": 0.6}, Label: "treat_A", Score: 0.0},
			// Edge cases for red-teaming (per DARPA requirement)
			{Features: map[string]float64{"blood_pressure": 0.5, "glucose": 0.5, "heart_rate": 0.5}, Label: "treat_B", Score: 0.0},   // boundary
			{Features: map[string]float64{"blood_pressure": 0.99, "glucose": 0.01, "heart_rate": 0.99}, Label: "treat_A", Score: 0.0}, // extreme
			{Features: map[string]float64{"blood_pressure": 0.01, "glucose": 0.99, "heart_rate": 0.01}, Label: "treat_B", Score: 0.0}, // extreme opposite
		},
	}
}

// COATrainSet returns a training dataset for Course of Action planning.
func COATrainSet() kinds.DataSet {
	return kinds.DataSet{
		Name:  "coa-planning-train",
		Split: "train",
		Items: []kinds.Datum{
			{Features: map[string]float64{"threat_level": 0.9, "supply_available": 0.3, "terrain_difficulty": 0.7}, Label: "defend", Score: 0.85},
			{Features: map[string]float64{"threat_level": 0.2, "supply_available": 0.9, "terrain_difficulty": 0.2}, Label: "advance", Score: 0.9},
			{Features: map[string]float64{"threat_level": 0.5, "supply_available": 0.5, "terrain_difficulty": 0.5}, Label: "hold", Score: 0.7},
			{Features: map[string]float64{"threat_level": 0.8, "supply_available": 0.8, "terrain_difficulty": 0.3}, Label: "flank", Score: 0.75},
			{Features: map[string]float64{"threat_level": 0.1, "supply_available": 0.1, "terrain_difficulty": 0.9}, Label: "retreat", Score: 0.88},
			{Features: map[string]float64{"threat_level": 0.7, "supply_available": 0.6, "terrain_difficulty": 0.4}, Label: "defend", Score: 0.8},
			{Features: map[string]float64{"threat_level": 0.3, "supply_available": 0.7, "terrain_difficulty": 0.6}, Label: "advance", Score: 0.82},
			{Features: map[string]float64{"threat_level": 0.6, "supply_available": 0.2, "terrain_difficulty": 0.8}, Label: "retreat", Score: 0.78},
		},
	}
}

// COATestSet returns a test dataset for COA evaluation.
func COATestSet() kinds.DataSet {
	return kinds.DataSet{
		Name:  "coa-planning-test",
		Split: "test",
		Items: []kinds.Datum{
			{Features: map[string]float64{"threat_level": 0.85, "supply_available": 0.25, "terrain_difficulty": 0.75}, Label: "defend", Score: 0.0},
			{Features: map[string]float64{"threat_level": 0.15, "supply_available": 0.85, "terrain_difficulty": 0.15}, Label: "advance", Score: 0.0},
			{Features: map[string]float64{"threat_level": 0.45, "supply_available": 0.55, "terrain_difficulty": 0.45}, Label: "hold", Score: 0.0},
			{Features: map[string]float64{"threat_level": 0.95, "supply_available": 0.05, "terrain_difficulty": 0.95}, Label: "retreat", Score: 0.0},
			// Edge cases
			{Features: map[string]float64{"threat_level": 0.5, "supply_available": 0.5, "terrain_difficulty": 0.5}, Label: "hold", Score: 0.0},
			{Features: map[string]float64{"threat_level": 0.01, "supply_available": 0.99, "terrain_difficulty": 0.01}, Label: "advance", Score: 0.0},
		},
	}
}

// SupplyChainTrainSet returns a training dataset for supply chain/logistics.
func SupplyChainTrainSet() kinds.DataSet {
	return kinds.DataSet{
		Name:  "supply-chain-train",
		Split: "train",
		Items: []kinds.Datum{
			{Features: map[string]float64{"equipment_age": 0.8, "usage_rate": 0.9, "failure_history": 0.7}, Label: "maintain_now", Score: 0.92},
			{Features: map[string]float64{"equipment_age": 0.2, "usage_rate": 0.3, "failure_history": 0.1}, Label: "no_action", Score: 0.95},
			{Features: map[string]float64{"equipment_age": 0.6, "usage_rate": 0.7, "failure_history": 0.5}, Label: "schedule_maint", Score: 0.8},
			{Features: map[string]float64{"equipment_age": 0.9, "usage_rate": 0.5, "failure_history": 0.9}, Label: "replace", Score: 0.85},
			{Features: map[string]float64{"equipment_age": 0.4, "usage_rate": 0.8, "failure_history": 0.3}, Label: "schedule_maint", Score: 0.78},
			{Features: map[string]float64{"equipment_age": 0.1, "usage_rate": 0.1, "failure_history": 0.0}, Label: "no_action", Score: 0.99},
		},
	}
}

// SupplyChainTestSet returns a test dataset for supply chain evaluation.
func SupplyChainTestSet() kinds.DataSet {
	return kinds.DataSet{
		Name:  "supply-chain-test",
		Split: "test",
		Items: []kinds.Datum{
			{Features: map[string]float64{"equipment_age": 0.75, "usage_rate": 0.85, "failure_history": 0.65}, Label: "maintain_now", Score: 0.0},
			{Features: map[string]float64{"equipment_age": 0.15, "usage_rate": 0.25, "failure_history": 0.05}, Label: "no_action", Score: 0.0},
			{Features: map[string]float64{"equipment_age": 0.95, "usage_rate": 0.45, "failure_history": 0.95}, Label: "replace", Score: 0.0},
			{Features: map[string]float64{"equipment_age": 0.5, "usage_rate": 0.5, "failure_history": 0.5}, Label: "schedule_maint", Score: 0.0},
		},
	}
}

// AllTrainSets returns all training datasets.
func AllTrainSets() []kinds.DataSet {
	return []kinds.DataSet{
		MedicalTrainSet(),
		COATrainSet(),
		SupplyChainTrainSet(),
	}
}

// AllTestSets returns all test datasets.
func AllTestSets() []kinds.DataSet {
	return []kinds.DataSet{
		MedicalTestSet(),
		COATestSet(),
		SupplyChainTestSet(),
	}
}
