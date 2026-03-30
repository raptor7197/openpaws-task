package llm

import (
	"context"
	"testing"

	"openpaws/internal/model"
)

func TestMockProviderStrongAnimalWelfare(t *testing.T) {
	t.Parallel()
	provider := MockProvider{}
	account := model.Account{
		Bio:           "Animal rescue stories, anti-cruelty advocacy, shelter fundraising.",
		TopicsClaimed: []string{"animal welfare", "rescue"},
		Posts: []model.ContentItem{
			{CaptionOrText: "Visited a local sanctuary today. Adopt don't shop."},
		},
	}
	class, err := provider.ClassifyAccount(context.Background(), "ban factory farming", account)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if class.AlignmentLabel != "strong_animal_welfare" {
		t.Fatalf("expected strong_animal_welfare, got %s", class.AlignmentLabel)
	}
}

func TestMockProviderHostile(t *testing.T) {
	t.Parallel()
	provider := MockProvider{}
	account := model.Account{
		Bio:           "Animal activists are extremists and anti-vegan commentary.",
		TopicsClaimed: []string{"commentary"},
		Posts: []model.ContentItem{
			{CaptionOrText: "Animal activists are extremists and factory farming criticism is fake outrage."},
		},
	}
	class, err := provider.ClassifyAccount(context.Background(), "animal welfare", account)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if class.AlignmentLabel != "misaligned_or_hostile" {
		t.Fatalf("expected misaligned_or_hostile, got %s", class.AlignmentLabel)
	}
	if !class.Hostile {
		t.Fatal("expected hostile flag")
	}
}

func TestMockProviderCommercial(t *testing.T) {
	t.Parallel()
	provider := MockProvider{}
	account := model.Account{
		Bio:           "Daily luxury routines, sponsored lifestyle picks.",
		TopicsClaimed: []string{"fashion"},
		Posts: []model.ContentItem{
			{CaptionOrText: "Use my discount code for today's sponsored drop. Shop now."},
			{CaptionOrText: "Trying a vegan cafe for one sponsored brunch collab."},
		},
	}
	class, err := provider.ClassifyAccount(context.Background(), "ban factory farming", account)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if class.AlignmentLabel != "commercial_only" {
		t.Fatalf("expected commercial_only, got %s", class.AlignmentLabel)
	}
	if !class.Opportunistic {
		t.Fatal("expected opportunistic flag")
	}
}

func TestMockProviderAdjacentCause(t *testing.T) {
	t.Parallel()
	provider := MockProvider{}
	account := model.Account{
		Bio:           "Climate, food systems, sustainability commentary.",
		TopicsClaimed: []string{"climate", "sustainability"},
		Posts: []model.ContentItem{
			{CaptionOrText: "Food systems reform must include sustainability."},
		},
	}
	class, err := provider.ClassifyAccount(context.Background(), "ban factory farming", account)
	if err != nil {
		t.Fatalf("classify: %v", err)
	}
	if class.AlignmentLabel != "adjacent_progressive_cause" {
		t.Fatalf("expected adjacent_progressive_cause, got %s", class.AlignmentLabel)
	}
}

func TestValidateClassificationValid(t *testing.T) {
	t.Parallel()
	c := model.Classification{
		AlignmentLabel:      "strong_animal_welfare",
		AlignmentConfidence: 0.90,
		ReceptivityScore:    0.82,
	}
	if err := validateClassification(c); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateClassificationMissingLabel(t *testing.T) {
	t.Parallel()
	c := model.Classification{
		AlignmentConfidence: 0.90,
	}
	if err := validateClassification(c); err == nil {
		t.Fatal("expected error for missing label")
	}
}

func TestValidateClassificationInvalidLabel(t *testing.T) {
	t.Parallel()
	c := model.Classification{
		AlignmentLabel:      "unknown_label",
		AlignmentConfidence: 0.90,
	}
	if err := validateClassification(c); err == nil {
		t.Fatal("expected error for invalid label")
	}
}

func TestValidateClassificationOutOfRange(t *testing.T) {
	t.Parallel()
	c := model.Classification{
		AlignmentLabel:      "strong_animal_welfare",
		AlignmentConfidence: 1.5,
	}
	if err := validateClassification(c); err == nil {
		t.Fatal("expected error for confidence > 1")
	}
}
