// Package testdata provides dummy test datasets for CARLA evaluation.
// Three DARPA-suggested application task domains:
// 1. Medical treatment decisions (multi-condition guidance)
// 2. Course of Action (COA) planning
// 3. Supply chain / predictive maintenance
package testdata

import "github.com/clara-phase1/kinds"

func MedicalTestSet() kinds.DataSet {
	return kinds.DataSet{
		Name: "medical-treatment-test", Split: "test",
		Items: []kinds.Datum{
			{Features: map[string]float64{"blood_pressure": 0.85, "glucose": 0.75, "heart_rate": 0.65}, Label: "treat_A"},
			{Features: map[string]float64{"blood_pressure": 0.25, "glucose": 0.85, "heart_rate": 0.35}, Label: "treat_B"},
			{Features: map[string]float64{"blood_pressure": 0.15, "glucose": 0.15, "heart_rate": 0.2}, Label: "no_treat"},
			{Features: map[string]float64{"blood_pressure": 0.9, "glucose": 0.9, "heart_rate": 0.85}, Label: "treat_C"},
			{Features: map[string]float64{"blood_pressure": 0.55, "glucose": 0.45, "heart_rate": 0.6}, Label: "treat_A"},
			// Edge cases for red-teaming
			{Features: map[string]float64{"blood_pressure": 0.5, "glucose": 0.5, "heart_rate": 0.5}, Label: "treat_B"},
			{Features: map[string]float64{"blood_pressure": 0.99, "glucose": 0.01, "heart_rate": 0.99}, Label: "treat_A"},
			{Features: map[string]float64{"blood_pressure": 0.01, "glucose": 0.99, "heart_rate": 0.01}, Label: "treat_B"},
		},
	}
}

func COATestSet() kinds.DataSet {
	return kinds.DataSet{
		Name: "coa-planning-test", Split: "test",
		Items: []kinds.Datum{
			{Features: map[string]float64{"threat_level": 0.85, "supply_available": 0.25, "terrain_difficulty": 0.75}, Label: "defend"},
			{Features: map[string]float64{"threat_level": 0.15, "supply_available": 0.85, "terrain_difficulty": 0.15}, Label: "advance"},
			{Features: map[string]float64{"threat_level": 0.45, "supply_available": 0.55, "terrain_difficulty": 0.45}, Label: "hold"},
			{Features: map[string]float64{"threat_level": 0.95, "supply_available": 0.05, "terrain_difficulty": 0.95}, Label: "retreat"},
			// Edge cases
			{Features: map[string]float64{"threat_level": 0.5, "supply_available": 0.5, "terrain_difficulty": 0.5}, Label: "hold"},
			{Features: map[string]float64{"threat_level": 0.01, "supply_available": 0.99, "terrain_difficulty": 0.01}, Label: "advance"},
		},
	}
}

func SupplyChainTestSet() kinds.DataSet {
	return kinds.DataSet{
		Name: "supply-chain-test", Split: "test",
		Items: []kinds.Datum{
			{Features: map[string]float64{"equipment_age": 0.75, "usage_rate": 0.85, "failure_history": 0.65}, Label: "maintain_now"},
			{Features: map[string]float64{"equipment_age": 0.15, "usage_rate": 0.25, "failure_history": 0.05}, Label: "no_action"},
			{Features: map[string]float64{"equipment_age": 0.95, "usage_rate": 0.45, "failure_history": 0.95}, Label: "replace"},
			{Features: map[string]float64{"equipment_age": 0.5, "usage_rate": 0.5, "failure_history": 0.5}, Label: "schedule_maint"},
		},
	}
}

func AllTestSets() []kinds.DataSet {
	return []kinds.DataSet{MedicalTestSet(), COATestSet(), SupplyChainTestSet()}
}
