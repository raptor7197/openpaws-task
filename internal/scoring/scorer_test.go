package scoring

import (
	"math"
	"testing"

	"openpaws/internal/config"
	"openpaws/internal/model"
)

func TestScoreAlignmentStrongAnimalWelfare(t *testing.T) {
	t.Parallel()
	class := model.Classification{
		AlignmentLabel:      "strong_animal_welfare",
		AlignmentConfidence: 0.90,
	}
	score, _, flags := scoreAlignment(class)
	if score != 0.95 {
		t.Fatalf("expected 0.95, got %f", score)
	}
	if len(flags) != 0 {
		t.Fatalf("expected no flags, got %v", flags)
	}
}

func TestScoreAlignmentHostileCapped(t *testing.T) {
	t.Parallel()
	class := model.Classification{
		AlignmentLabel:      "adjacent_progressive_cause",
		AlignmentConfidence: 0.72,
		Hostile:             true,
	}
	score, _, flags := scoreAlignment(class)
	if score > 0.05 {
		t.Fatalf("expected hostile cap at 0.05, got %f", score)
	}
	found := false
	for _, f := range flags {
		if f == "hostile" {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected hostile flag, got %v", flags)
	}
}

func TestScoreAlignmentCommercialFlagged(t *testing.T) {
	t.Parallel()
	class := model.Classification{
		AlignmentLabel:      "commercial_only",
		AlignmentConfidence: 0.76,
		Opportunistic:       true,
	}
	score, _, flags := scoreAlignment(class)
	if score > 0.15 {
		t.Fatalf("expected low score for commercial+opportunistic, got %f", score)
	}
	hasMisaligned := false
	hasOpportunistic := false
	for _, f := range flags {
		if f == "misaligned" {
			hasMisaligned = true
		}
		if f == "opportunistic" {
			hasOpportunistic = true
		}
	}
	if !hasMisaligned || !hasOpportunistic {
		t.Fatalf("expected misaligned+opportunistic flags, got %v", flags)
	}
}

func TestScoreAuthenticityOrganic(t *testing.T) {
	t.Parallel()
	account := model.Account{
		FollowerCount: 42000,
		Platform:      model.PlatformInstagram,
		Posts: []model.ContentItem{
			{LikeCount: 3200, CommentCount: 320, ShareCount: 210},
			{LikeCount: 2900, CommentCount: 280, ShareCount: 160},
			{LikeCount: 2500, CommentCount: 250, ShareCount: 150},
		},
		Comments: []model.CommentSample{
			{Text: "Thank you for helping local shelters."},
			{Text: "We adopted our dog after following your rescue tips."},
			{Text: "The farm animal segment was powerful and informative."},
			{Text: "Love how clearly you explain anti-cruelty policy."},
			{Text: "Sharing this with our foster group."},
			{Text: "Can we volunteer at the adoption event?"},
			{Text: "Your posts helped us find a rescue partner."},
			{Text: "This is exactly the advocacy content we need."},
		},
		GrowthSnapshots: []model.GrowthSnapshot{
			{FollowerCount: 39000},
			{FollowerCount: 40500},
			{FollowerCount: 42000},
		},
	}
	score, _, flags := scoreAuthenticity(account)
	if score < 0.70 {
		t.Fatalf("expected healthy authenticity, got %f", score)
	}
	for _, f := range flags {
		if f == "engagement_suspicious" || f == "growth_suspicious" {
			t.Fatalf("organic account should not have suspicious flags, got %v", flags)
		}
	}
}

func TestScoreAuthenticityBotComments(t *testing.T) {
	t.Parallel()
	account := model.Account{
		FollowerCount: 210000,
		Platform:      model.PlatformX,
		PostCount:     1500,
		Posts: []model.ContentItem{
			{ContentID: "p1", LikeCount: 220, CommentCount: 3, ShareCount: 2},
			{ContentID: "p2", LikeCount: 180, CommentCount: 2, ShareCount: 1},
		},
		Comments: []model.CommentSample{
			{ContentID: "p1", AuthorHandle: "bot1", Text: "nice"},
			{ContentID: "p1", AuthorHandle: "bot2", Text: "nice"},
			{ContentID: "p1", AuthorHandle: "bot3", Text: "nice"},
			{ContentID: "p2", AuthorHandle: "bot4", Text: "🔥🔥🔥"},
			{ContentID: "p2", AuthorHandle: "bot5", Text: "nice"},
			{ContentID: "p2", AuthorHandle: "bot6", Text: "wow"},
		},
		GrowthSnapshots: []model.GrowthSnapshot{
			{FollowerCount: 70000},
			{FollowerCount: 73000},
			{FollowerCount: 210000},
		},
	}
	score, _, flags := scoreAuthenticity(account)
	if score > 0.30 {
		t.Fatalf("expected low authenticity for bot-heavy account, got %f", score)
	}
	hasSuspicious := false
	for _, f := range flags {
		if f == "engagement_suspicious" || f == "growth_suspicious" {
			hasSuspicious = true
		}
	}
	if !hasSuspicious {
		t.Fatalf("expected suspicious flags, got %v", flags)
	}
}

func TestScoreReachNoNaN(t *testing.T) {
	t.Parallel()
	cfg := config.Default()

	// Small dataset: 2 accounts on same platform, guaranteeing P90 == P95.
	accounts := []model.Account{
		{
			Platform:      model.PlatformInstagram,
			FollowerCount: 42000,
			Posts: []model.ContentItem{
				{LikeCount: 3200, CommentCount: 320, ShareCount: 210},
			},
		},
		{
			Platform:      model.PlatformInstagram,
			FollowerCount: 180000,
			Posts: []model.ContentItem{
				{LikeCount: 1400, CommentCount: 18, ShareCount: 5},
			},
		},
	}

	stats := BuildDatasetStats(accounts)
	for _, account := range accounts {
		reach, _ := scoreReach(cfg, stats, account)
		if math.IsNaN(reach) {
			t.Fatalf("reach score for %d followers should not be NaN", account.FollowerCount)
		}
		if math.IsInf(reach, 0) {
			t.Fatalf("reach score for %d followers should not be Inf", account.FollowerCount)
		}
		if reach < 0 || reach > 1 {
			t.Fatalf("reach score should be in [0,1], got %f", reach)
		}
	}
}

func TestScoreReachSingleAccount(t *testing.T) {
	t.Parallel()
	cfg := config.Default()

	accounts := []model.Account{
		{
			Platform:      model.PlatformX,
			FollowerCount: 28000,
			Posts: []model.ContentItem{
				{LikeCount: 650, CommentCount: 90, ShareCount: 70},
			},
		},
	}

	stats := BuildDatasetStats(accounts)
	reach, _ := scoreReach(cfg, stats, accounts[0])
	if math.IsNaN(reach) || math.IsInf(reach, 0) {
		t.Fatalf("single-account reach score should not be NaN/Inf, got %f", reach)
	}
}

func TestScoreConfidenceSparseData(t *testing.T) {
	t.Parallel()
	account := model.Account{
		Posts:    []model.ContentItem{{CaptionOrText: "single post"}},
		Comments: []model.CommentSample{{Text: "one comment"}},
	}
	class := model.Classification{
		AlignmentConfidence: 0.50,
	}
	confidence := scoreConfidence(account, class)
	if confidence > 0.45 {
		t.Fatalf("sparse account should have low confidence, got %f", confidence)
	}
}

func TestScoreConfidenceRichData(t *testing.T) {
	t.Parallel()
	account := model.Account{
		Posts: []model.ContentItem{{}, {}, {}},
		Comments: []model.CommentSample{
			{Text: "1"}, {Text: "2"}, {Text: "3"}, {Text: "4"},
			{Text: "5"}, {Text: "6"}, {Text: "7"}, {Text: "8"},
		},
		GrowthSnapshots: []model.GrowthSnapshot{{}, {}, {}},
	}
	class := model.Classification{
		AlignmentConfidence: 0.90,
	}
	confidence := scoreConfidence(account, class)
	if confidence < 0.80 {
		t.Fatalf("rich-data account should have high confidence, got %f", confidence)
	}
}

func TestCompositeScoreWeighted(t *testing.T) {
	t.Parallel()
	cfg := config.Default()

	accounts := []model.Account{
		{
			Platform:      model.PlatformInstagram,
			FollowerCount: 42000,
			PostCount:     520,
			Posts: []model.ContentItem{
				{LikeCount: 3200, CommentCount: 320, ShareCount: 210},
				{LikeCount: 2900, CommentCount: 280, ShareCount: 160},
				{LikeCount: 2500, CommentCount: 250, ShareCount: 150},
			},
			Comments: []model.CommentSample{
				{Text: "Great advocacy work!"},
				{Text: "Love this content"},
				{Text: "Really helpful info"},
				{Text: "Keep it up!"},
				{Text: "Amazing rescue story"},
				{Text: "This is important work"},
				{Text: "Thank you for sharing"},
				{Text: "Inspirational content"},
			},
			GrowthSnapshots: []model.GrowthSnapshot{
				{FollowerCount: 39000},
				{FollowerCount: 40500},
				{FollowerCount: 42000},
			},
		},
	}

	stats := BuildDatasetStats(accounts)
	class := model.Classification{
		AlignmentLabel:      "strong_animal_welfare",
		AlignmentConfidence: 0.90,
		ReceptivityLabel:    "high",
		ReceptivityScore:    0.82,
	}

	result := ScoreAccount(cfg, stats, accounts[0], class)

	if result.CompositeScore <= 0 {
		t.Fatalf("composite should be positive, got %f", result.CompositeScore)
	}
	if math.IsNaN(result.CompositeScore) {
		t.Fatal("composite score should not be NaN")
	}

	// Verify it respects weights: alignment=0.40, authenticity=0.25, receptivity=0.20, reach=0.15
	expected := (result.CauseAlignmentScore * 0.40) +
		(result.EngagementAuthenticityScore * 0.25) +
		(result.ReceptivityScore * 0.20) +
		(result.ReachScore * 0.15)
	diff := math.Abs(result.CompositeScore - expected)
	if diff > 0.05 {
		t.Fatalf("composite %.3f does not match expected %.3f (diff %.3f)", result.CompositeScore, expected, diff)
	}
}

func TestRecommendationContact(t *testing.T) {
	t.Parallel()
	rec, _ := computeRecommendation(0.90, 0.85, 0.90, nil)
	if rec != model.RecommendationContact {
		t.Fatalf("expected contact recommendation, got %s", rec)
	}
}

func TestRecommendationAvoidHostile(t *testing.T) {
	t.Parallel()
	rec, _ := computeRecommendation(0.90, 0.85, 0.90, []string{"hostile"})
	if rec != model.RecommendationAvoid {
		t.Fatalf("expected avoid for hostile, got %s", rec)
	}
}

func TestRecommendationReviewLowConfidence(t *testing.T) {
	t.Parallel()
	rec, _ := computeRecommendation(0.80, 0.80, 0.30, nil)
	if rec != model.RecommendationReview {
		t.Fatalf("expected review for low confidence, got %s", rec)
	}
}

func TestClampHandlesNaN(t *testing.T) {
	t.Parallel()
	if clamp(math.NaN()) != 0 {
		t.Fatal("NaN should clamp to 0")
	}
	if clamp(math.Inf(1)) != 0 {
		t.Fatal("+Inf should clamp to 0")
	}
	if clamp(math.Inf(-1)) != 0 {
		t.Fatal("-Inf should clamp to 0")
	}
	if clamp(-0.5) != 0 {
		t.Fatal("negative should clamp to 0")
	}
	if clamp(1.5) != 1 {
		t.Fatal(">1 should clamp to 1")
	}
}

func TestFollowerGrowthSpikeDetection(t *testing.T) {
	t.Parallel()
	snapshots := []model.GrowthSnapshot{
		{FollowerCount: 70000},
		{FollowerCount: 73000},
		{FollowerCount: 210000},
	}
	spike := followerGrowthSpike(snapshots)
	if spike < 1.0 {
		t.Fatalf("expected major growth spike, got %f", spike)
	}
}

func TestEmojiLikeCommentRatio(t *testing.T) {
	t.Parallel()
	comments := []model.CommentSample{
		{Text: "nice"},
		{Text: "wow"},
		{Text: "🔥🔥🔥"},
		{Text: "This is a thoughtful analysis of the issue"},
	}
	ratio := emojiLikeCommentRatio(comments)
	if ratio < 0.50 {
		t.Fatalf("expected high low-info ratio, got %f", ratio)
	}
}

func TestRepetitiveCommentRatio(t *testing.T) {
	t.Parallel()
	comments := []model.CommentSample{
		{Text: "nice"},
		{Text: "nice"},
		{Text: "nice"},
		{Text: "great content"},
	}
	ratio := repetitiveCommentRatio(comments)
	if ratio < 0.50 {
		t.Fatalf("expected high repetitive ratio, got %f", ratio)
	}
}

func TestUniqueCommentRatioAllUnique(t *testing.T) {
	t.Parallel()
	comments := []model.CommentSample{
		{Text: "Great advocacy work!"},
		{Text: "Really helpful information"},
		{Text: "Thank you for sharing this"},
		{Text: "I learned a lot from this"},
	}
	ratio := uniqueCommentRatio(comments)
	if ratio != 1.0 {
		t.Fatalf("expected 1.0 unique ratio, got %f", ratio)
	}
}
