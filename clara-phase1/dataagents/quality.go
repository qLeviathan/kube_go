package dataagents

import (
	"fmt"
	"math"
	"sort"

	"github.com/clara-phase1/kinds"
)

// QualityScore summarizes data quality for a dataset.
type QualityScore struct {
	DatasetName     string
	OverallScore    float64 // 0.0 to 1.0
	CompletenessScore float64
	ConsistencyScore  float64
	BalanceScore      float64
	RangeScore        float64
	OutlierCount      int
	Details           []string
}

// QualityAgent assesses data quality: outliers, balance, completeness, distributions.
type QualityAgent struct {
	AgentID string
	Log     []string
}

func NewQualityAgent(id string) *QualityAgent {
	return &QualityAgent{AgentID: id}
}

func (q *QualityAgent) ID() string { return q.AgentID }

// Assess runs a full quality assessment on a dataset.
func (q *QualityAgent) Assess(ds kinds.DataSet) QualityScore {
	q.log("assessing quality: %s (%d items)", ds.Name, len(ds.Items))

	score := QualityScore{DatasetName: ds.Name}

	if len(ds.Items) == 0 {
		score.Details = append(score.Details, "empty dataset — cannot assess")
		return score
	}

	// 1. Completeness: all items have features and labels
	complete := 0
	for i := 0; i < len(ds.Items); i++ {
		if len(ds.Items[i].Features) > 0 && ds.Items[i].Label != "" {
			complete++
		}
	}
	score.CompletenessScore = float64(complete) / float64(len(ds.Items))
	score.Details = append(score.Details,
		fmt.Sprintf("completeness: %.1f%% (%d/%d items fully specified)",
			score.CompletenessScore*100, complete, len(ds.Items)))

	// 2. Consistency: all items have same feature count
	expectedFeatures := len(ds.Items[0].Features)
	consistent := 0
	for i := 0; i < len(ds.Items); i++ {
		if len(ds.Items[i].Features) == expectedFeatures {
			consistent++
		}
	}
	score.ConsistencyScore = float64(consistent) / float64(len(ds.Items))
	score.Details = append(score.Details,
		fmt.Sprintf("consistency: %.1f%% (%d/%d items have %d features)",
			score.ConsistencyScore*100, consistent, len(ds.Items), expectedFeatures))

	// 3. Balance: class distribution evenness
	labelCounts := make(map[string]int)
	for i := 0; i < len(ds.Items); i++ {
		labelCounts[ds.Items[i].Label]++
	}
	if len(labelCounts) > 1 {
		counts := make([]int, 0, len(labelCounts))
		for _, c := range labelCounts {
			counts = append(counts, c)
		}
		sort.Ints(counts)
		minC := float64(counts[0])
		maxC := float64(counts[len(counts)-1])
		score.BalanceScore = minC / maxC
		score.Details = append(score.Details,
			fmt.Sprintf("balance: %.1f%% (min class=%d, max class=%d, %d classes)",
				score.BalanceScore*100, counts[0], counts[len(counts)-1], len(labelCounts)))
	} else {
		score.BalanceScore = 0.0
		score.Details = append(score.Details, "balance: single class — no variance")
	}

	// 4. Range: features within [0,1]
	totalFeatureValues := 0
	inRange := 0
	for i := 0; i < len(ds.Items); i++ {
		for _, v := range ds.Items[i].Features {
			totalFeatureValues++
			if v >= 0 && v <= 1 {
				inRange++
			}
		}
	}
	if totalFeatureValues > 0 {
		score.RangeScore = float64(inRange) / float64(totalFeatureValues)
	}
	score.Details = append(score.Details,
		fmt.Sprintf("range compliance: %.1f%% of feature values in [0,1]", score.RangeScore*100))

	// 5. Outlier detection (IQR method per feature)
	featureVals := make(map[string][]float64)
	for i := 0; i < len(ds.Items); i++ {
		for fname, fval := range ds.Items[i].Features {
			featureVals[fname] = append(featureVals[fname], fval)
		}
	}

	for fname, vals := range featureVals {
		sort.Float64s(vals)
		n := len(vals)
		if n < 4 {
			continue
		}
		q1 := vals[n/4]
		q3 := vals[3*n/4]
		iqr := q3 - q1
		lower := q1 - 1.5*iqr
		upper := q3 + 1.5*iqr

		outliers := 0
		for _, v := range vals {
			if v < lower || v > upper {
				outliers++
			}
		}
		if outliers > 0 {
			score.OutlierCount += outliers
			score.Details = append(score.Details,
				fmt.Sprintf("outliers in %s: %d values outside [%.2f, %.2f]",
					fname, outliers, lower, upper))
		}
	}

	// Overall score: weighted average
	score.OverallScore = (score.CompletenessScore*0.3 +
		score.ConsistencyScore*0.2 +
		score.BalanceScore*0.2 +
		score.RangeScore*0.3)
	score.OverallScore = math.Max(0, math.Min(1, score.OverallScore))

	q.log("quality assessment: overall=%.2f completeness=%.2f consistency=%.2f balance=%.2f range=%.2f outliers=%d",
		score.OverallScore, score.CompletenessScore, score.ConsistencyScore,
		score.BalanceScore, score.RangeScore, score.OutlierCount)

	return score
}

func (q *QualityAgent) log(format string, args ...interface{}) {
	q.Log = append(q.Log, fmt.Sprintf(format, args...))
}
