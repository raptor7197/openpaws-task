package llm

import (
	"context"
	"strings"

	"openpaws/internal/model"
)

type MockProvider struct{}

func (MockProvider) ClassifyAccount(_ context.Context, topic string, account model.Account) (model.Classification, error) {
	text := strings.ToLower(account.Bio + " " + strings.Join(account.TopicsClaimed, " ") + " " + collectPosts(account))
	topic = strings.ToLower(topic)
	strongCauseHits := countTerms(text, "animal welfare", "rescue", "adopt don't shop", "anti-cruelty", "sanctuary", "factory farming")
	adjacentHits := countTerms(text, "sustainability", "climate", "ethical food", "environment", topic)
	commercialHits := countTerms(text, "sponsored", "discount code", "shop now", "dropshipping", "promo")
	hostileHits := countTerms(text, "anti-vegan", "hunting trophy", "cruelty is fake", "animal activists are extremists")

	classification := model.Classification{
		AlignmentLabel:      "neutral_general_interest",
		AlignmentConfidence: 0.55,
		ReceptivityLabel:    "low",
		ReceptivityScore:    0.35,
		Rationale:           []string{"Limited direct evidence of sustained cause advocacy."},
	}

	// The mock provider behaves like a deterministic stand-in for a real LLM so
	// the rest of the pipeline can be tested offline and remain reproducible.
	switch {
	case hostileHits > 0:
		classification.AlignmentLabel = "misaligned_or_hostile"
		classification.AlignmentConfidence = 0.88
		classification.ReceptivityLabel = "very_low"
		classification.ReceptivityScore = 0.05
		classification.Hostile = true
		classification.Rationale = []string{
			"Account signals direct hostility or opposition to the campaign topic.",
		}
	case commercialHits >= 2 && strongCauseHits == 0:
		classification.AlignmentLabel = "commercial_only"
		classification.AlignmentConfidence = 0.76
		classification.ReceptivityLabel = "low"
		classification.ReceptivityScore = 0.28
		classification.Opportunistic = true
		classification.Rationale = []string{
			"Commercial messaging dominates the account identity and weakens mission fit.",
		}
	case strongCauseHits >= 2:
		classification.AlignmentLabel = "strong_animal_welfare"
		classification.AlignmentConfidence = 0.90
		classification.ReceptivityLabel = "high"
		classification.ReceptivityScore = 0.82
		classification.Rationale = []string{
			"Content repeatedly references animal rescue, cruelty prevention, or ethical treatment themes.",
			"Audience-facing tone suggests openness to mission-driven outreach.",
		}
	case strongCauseHits == 1 || adjacentHits > 0:
		classification.AlignmentLabel = "adjacent_progressive_cause"
		classification.AlignmentConfidence = 0.72
		classification.ReceptivityLabel = "medium"
		classification.ReceptivityScore = 0.63
		classification.Rationale = []string{
			"Account appears adjacent to the target cause without showing the same depth of focus.",
		}
	}

	return classification, nil
}

func collectPosts(account model.Account) string {
	var items []string
	for i, post := range account.Posts {
		if i >= 8 {
			break
		}
		items = append(items, post.CaptionOrText)
	}
	return strings.Join(items, " ")
}

func containsAny(text string, terms ...string) bool {
	for _, term := range terms {
		if strings.Contains(text, strings.ToLower(term)) {
			return true
		}
	}
	return false
}

func countTerms(text string, terms ...string) int {
	count := 0
	for _, term := range terms {
		term = strings.TrimSpace(strings.ToLower(term))
		if term == "" {
			continue
		}
		if strings.Contains(text, term) {
			count++
		}
	}
	return count
}
