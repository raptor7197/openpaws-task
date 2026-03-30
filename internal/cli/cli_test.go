package cli

import (
	"strings"
	"testing"

	"openpaws/internal/model"
)

func TestParsePlatforms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		input    string
		expected []model.Platform
	}{
		{"instagram,x", []model.Platform{model.PlatformInstagram, model.PlatformX}},
		{"instagram", []model.Platform{model.PlatformInstagram}},
		{"x", []model.Platform{model.PlatformX}},
		{"Instagram, X", []model.Platform{model.PlatformInstagram, model.PlatformX}},
		{"unknown", nil},
		{"", nil},
	}

	for _, tt := range tests {
		result := ParsePlatforms(tt.input)
		if len(result) != len(tt.expected) {
			t.Errorf("ParsePlatforms(%q) = %v, want %v", tt.input, result, tt.expected)
		}
	}
}

func TestFormatConsoleReport(t *testing.T) {
	t.Parallel()

	report := model.Report{
		Topic: "ban factory farming",
		Results: []model.ScoredAccount{
			{
				Account: model.Account{
					Handle:   "@testaccount",
					Platform: model.PlatformInstagram,
				},
				CompositeScore:              0.832,
				CauseAlignmentScore:         0.95,
				EngagementAuthenticityScore: 0.85,
				ReceptivityScore:            0.82,
				ReachScore:                  0.502,
				Flags:                       []string{"test_flag"},
				Evidence:                    []string{"Evidence line 1"},
			},
		},
	}

	output := FormatConsoleReport(report)

	if !strings.Contains(output, "ban factory farming") {
		t.Error("expected topic in output")
	}
	if !strings.Contains(output, "@testaccount") {
		t.Error("expected handle in output")
	}
	if !strings.Contains(output, "0.832") {
		t.Error("expected composite score in output")
	}
	if !strings.Contains(output, "test_flag") {
		t.Error("expected flags in output")
	}
	if !strings.Contains(output, "Evidence line 1") {
		t.Error("expected evidence in output")
	}
}

func TestFormatConsoleReportEmpty(t *testing.T) {
	t.Parallel()
	report := model.Report{
		Topic:   "test",
		Results: nil,
	}
	output := FormatConsoleReport(report)
	if !strings.Contains(output, "Accounts Ranked: 0") {
		t.Error("expected zero accounts in output")
	}
}
