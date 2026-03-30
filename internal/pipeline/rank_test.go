package pipeline_test

import (
	"context"
	"path/filepath"
	"strings"
	"testing"

	"openpaws/internal/config"
	"openpaws/internal/llm"
	"openpaws/internal/model"
	"openpaws/internal/pipeline"
)

func TestRankFlagsInauthenticAccountAndRanksAlignedHigher(t *testing.T) {
	t.Parallel()

	runner := pipeline.Runner{
		Config:      config.Default(),
		LLMProvider: llm.MockProvider{},
	}

	report, err := runner.Rank(context.Background(), pipeline.RankRequest{
		Topic:     "ban factory farming",
		InputDir:  filepath.Join("..", "..", "testdata", "fixtures"),
		Platforms: []model.Platform{model.PlatformInstagram, model.PlatformX},
	})
	if err != nil {
		t.Fatalf("rank: %v", err)
	}

	if len(report.Results) < 3 {
		t.Fatalf("expected ranked results, got %d", len(report.Results))
	}

	if got := report.Results[0].Account.Handle; got != "@rescuevoices" {
		t.Fatalf("expected top ranked aligned account, got %s", got)
	}

	var suspicious model.ScoredAccount
	for _, result := range report.Results {
		if result.Account.Handle == "@viralpetbuzz" {
			suspicious = result
			break
		}
	}

	if suspicious.Account.Handle == "" {
		t.Fatal("expected suspicious account to be present")
	}
	if !contains(suspicious.Flags, "engagement_suspicious") && !contains(suspicious.Flags, "growth_suspicious") {
		t.Fatalf("expected suspicious flags for %s, got %v", suspicious.Account.Handle, suspicious.Flags)
	}
}

func TestRankPenalizesHostileAccount(t *testing.T) {
	t.Parallel()

	runner := pipeline.Runner{
		Config:      config.Default(),
		LLMProvider: llm.MockProvider{},
	}

	report, err := runner.Rank(context.Background(), pipeline.RankRequest{
		Topic:     "animal welfare outreach",
		InputDir:  filepath.Join("..", "..", "testdata", "fixtures"),
		Platforms: []model.Platform{model.PlatformX},
	})
	if err != nil {
		t.Fatalf("rank: %v", err)
	}

	for _, result := range report.Results {
		if result.Account.Handle == "@antiveganvoice" {
			if result.CauseAlignmentScore > 0.10 {
				t.Fatalf("expected hostile account to be heavily penalized, got %.3f", result.CauseAlignmentScore)
			}
			if !contains(result.Flags, "hostile") {
				t.Fatalf("expected hostile flag, got %v", result.Flags)
			}
			return
		}
	}

	t.Fatal("expected hostile account in results")
}

func contains(items []string, target string) bool {
	for _, item := range items {
		if strings.EqualFold(item, target) {
			return true
		}
	}
	return false
}
