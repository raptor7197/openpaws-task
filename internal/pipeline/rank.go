package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"openpaws/internal/config"
	"openpaws/internal/connector"
	"openpaws/internal/llm"
	"openpaws/internal/model"
	"openpaws/internal/scoring"
)

type RankRequest struct {
	Topic     string
	InputDir  string
	Output    string
	Platforms []model.Platform
}

type Runner struct {
	Config      config.Config
	LLMProvider llm.Provider
}

func (r Runner) Rank(ctx context.Context, req RankRequest) (model.Report, error) {
	loader := connector.FixtureLoader{}
	accounts, err := loader.Load(req.InputDir, req.Platforms)
	if err != nil {
		return model.Report{}, err
	}

	stats := scoring.BuildDatasetStats(accounts)
	results := make([]model.ScoredAccount, 0, len(accounts))

	for _, account := range accounts {
		// Classification is isolated behind a provider interface so teams can swap
		// mock, OpenAI, or future hosted models without changing scoring code.
		classification, err := r.LLMProvider.ClassifyAccount(ctx, req.Topic, account)
		if err != nil {
			return model.Report{}, fmt.Errorf("classify %s: %w", account.Handle, err)
		}

		results = append(results, scoring.ScoreAccount(r.Config, stats, account, classification))
	}

	sort.Slice(results, func(i, j int) bool {
		if results[i].CompositeScore == results[j].CompositeScore {
			return results[i].ConfidenceScore > results[j].ConfidenceScore
		}
		return results[i].CompositeScore > results[j].CompositeScore
	})

	report := model.Report{
		Topic:   req.Topic,
		Results: results,
	}

	if req.Output != "" {
		if err := writeReport(req.Output, report); err != nil {
			return model.Report{}, err
		}
	}

	return report, nil
}

func writeReport(path string, report model.Report) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal report: %w", err)
	}

	// Creating the parent directory makes the CLI safer for automation jobs that
	// write into timestamped report folders which may not exist yet.
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create report directory: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write report: %w", err)
	}
	return nil
}
