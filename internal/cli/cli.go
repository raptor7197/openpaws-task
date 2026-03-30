package cli

import (
	"errors"
	"fmt"
	"sort"
	"strings"

	"openpaws/internal/model"
)

var ErrUsage = errors.New("usage: cred-engine rank --topic <topic> --input <dir> [--output <file>]")

func ParsePlatforms(raw string) []model.Platform {
	parts := strings.Split(raw, ",")
	platforms := make([]model.Platform, 0, len(parts))
	for _, part := range parts {
		switch strings.TrimSpace(strings.ToLower(part)) {
		case string(model.PlatformInstagram):
			platforms = append(platforms, model.PlatformInstagram)
		case string(model.PlatformX):
			platforms = append(platforms, model.PlatformX)
		}
	}
	return platforms
}

func FormatConsoleReport(report model.Report) string {
	var b strings.Builder

	// Sorting again at presentation time avoids accidental display drift if a
	// caller modifies the slice order after ranking.
	results := append([]model.ScoredAccount(nil), report.Results...)
	sort.Slice(results, func(i, j int) bool {
		return results[i].CompositeScore > results[j].CompositeScore
	})

	fmt.Fprintf(&b, "Campaign Topic: %s\n", report.Topic)
	fmt.Fprintf(&b, "Accounts Ranked: %d\n\n", len(results))
	for idx, result := range results {
		fmt.Fprintf(&b, "%d. %s (%s)\n", idx+1, result.Account.Handle, result.Account.Platform)
		fmt.Fprintf(&b, "   Composite: %.3f | Alignment: %.3f | Authenticity: %.3f | Receptivity: %.3f | Reach: %.3f\n",
			result.CompositeScore,
			result.CauseAlignmentScore,
			result.EngagementAuthenticityScore,
			result.ReceptivityScore,
			result.ReachScore,
		)
		if len(result.Flags) > 0 {
			fmt.Fprintf(&b, "   Flags: %s\n", strings.Join(result.Flags, ", "))
		}
		if len(result.Evidence) > 0 {
			fmt.Fprintf(&b, "   Evidence: %s\n", strings.Join(result.Evidence, " | "))
		}
		b.WriteString("\n")
	}

	return b.String()
}
