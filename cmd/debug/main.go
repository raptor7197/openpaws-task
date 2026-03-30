package main

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"os"
	"path/filepath"

	"openpaws/internal/config"
	"openpaws/internal/llm"
	"openpaws/internal/model"
	"openpaws/internal/pipeline"
)

func main() {
	runner := pipeline.Runner{
		Config:      config.Default(),
		LLMProvider: llm.MockProvider{},
	}

	report, err := runner.Rank(context.Background(), pipeline.RankRequest{
		Topic:     "ban factory farming",
		InputDir:  filepath.Join("testdata", "fixtures"),
		Platforms: []model.Platform{model.PlatformInstagram, model.PlatformX},
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "rank error: %v\n", err)
		os.Exit(1)
	}

	for _, r := range report.Results {
		isNaN := math.IsNaN(r.ReachScore) || math.IsNaN(r.CompositeScore) || math.IsNaN(r.ConfidenceScore) ||
			math.IsNaN(r.CauseAlignmentScore) || math.IsNaN(r.EngagementAuthenticityScore) || math.IsNaN(r.ReceptivityScore)
		data, err := json.Marshal(r.ReachScore)
		fmt.Printf("Handle: %s, Reach: %f, NaN? %v, JSON err: %v, data: %s\n", r.Account.Handle, r.ReachScore, isNaN, err, string(data))
		fmt.Printf("  Alignment: %f, Authenticity: %f, Receptivity: %f, Composite: %f, Confidence: %f\n",
			r.CauseAlignmentScore, r.EngagementAuthenticityScore, r.ReceptivityScore, r.CompositeScore, r.ConfidenceScore)
	}
}
