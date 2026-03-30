package scoring

import (
	"fmt"
	"math"
	"slices"
	"sort"
	"strings"

	"openpaws/internal/config"
	"openpaws/internal/model"
)

type DatasetStats struct {
	MaxFollowers          map[model.Platform]int
	MaxEngagements        map[model.Platform]float64
	FollowerPercentiles   map[model.Platform]percentileSet
	EngagementPercentiles map[model.Platform]percentileSet
}

type percentileSet struct {
	P50 float64
	P75 float64
	P90 float64
	P95 float64
}

func BuildDatasetStats(accounts []model.Account) DatasetStats {
	stats := DatasetStats{
		MaxFollowers:          make(map[model.Platform]int),
		MaxEngagements:        make(map[model.Platform]float64),
		FollowerPercentiles:   make(map[model.Platform]percentileSet),
		EngagementPercentiles: make(map[model.Platform]percentileSet),
	}

	followerBuckets := map[model.Platform][]int{}
	engagementBuckets := map[model.Platform][]float64{}

	for _, account := range accounts {
		if account.FollowerCount > stats.MaxFollowers[account.Platform] {
			stats.MaxFollowers[account.Platform] = account.FollowerCount
		}
		avg := averageEngagement(account)
		if avg > stats.MaxEngagements[account.Platform] {
			stats.MaxEngagements[account.Platform] = avg
		}
		followerBuckets[account.Platform] = append(followerBuckets[account.Platform], account.FollowerCount)
		engagementBuckets[account.Platform] = append(engagementBuckets[account.Platform], avg)
	}

	for platform := range stats.MaxFollowers {
		if len(followerBuckets[platform]) > 0 {
			sort.Ints(followerBuckets[platform])
			stats.FollowerPercentiles[platform] = calculatePercentilesInt(followerBuckets[platform])
		}
		if len(engagementBuckets[platform]) > 0 {
			sort.Float64s(engagementBuckets[platform])
			stats.EngagementPercentiles[platform] = calculatePercentilesFloat(engagementBuckets[platform])
		}
	}

	return stats
}

func percentileIndex(n int, p float64) int {
	idx := int(float64(n-1) * p)
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return idx
}

func calculatePercentilesInt(sorted []int) percentileSet {
	n := len(sorted)
	if n == 0 {
		return percentileSet{}
	}
	return percentileSet{
		P50: float64(sorted[percentileIndex(n, 0.50)]),
		P75: float64(sorted[percentileIndex(n, 0.75)]),
		P90: float64(sorted[percentileIndex(n, 0.90)]),
		P95: float64(sorted[percentileIndex(n, 0.95)]),
	}
}

func calculatePercentilesFloat(sorted []float64) percentileSet {
	n := len(sorted)
	if n == 0 {
		return percentileSet{}
	}
	return percentileSet{
		P50: sorted[percentileIndex(n, 0.50)],
		P75: sorted[percentileIndex(n, 0.75)],
		P90: sorted[percentileIndex(n, 0.90)],
		P95: sorted[percentileIndex(n, 0.95)],
	}
}

func ScoreAccount(cfg config.Config, stats DatasetStats, account model.Account, class model.Classification) model.ScoredAccount {
	alignment, alignmentEvidence, alignmentFlags := scoreAlignment(class)
	authenticity, authenticityEvidence, authenticityFlags := scoreAuthenticity(account)
	receptivity, receptivityEvidence := scoreReceptivity(class)
	reach, reachEvidence := scoreReach(cfg, stats, account)
	confidence := scoreConfidence(account, class)

	flags := append(alignmentFlags, authenticityFlags...)
	evidence := append(alignmentEvidence, authenticityEvidence...)
	evidence = append(evidence, receptivityEvidence...)
	evidence = append(evidence, reachEvidence...)
	evidence = slices.Compact(evidence)

	composite := (alignment * cfg.Weights.CauseAlignment) +
		(authenticity * cfg.Weights.Authenticity) +
		(receptivity * cfg.Weights.Receptivity) +
		(reach * cfg.Weights.Reach)

	// Low-confidence accounts remain in the ranking, but they are softened so
	// sparse datasets do not outrank well-supported profiles too easily.
	if confidence < cfg.LowConfidenceFloor {
		flags = append(flags, "low_confidence", "manual_review_required")
		composite *= 0.90
	}

	recommendation, confidenceReasons := computeRecommendation(alignment, authenticity, confidence, flags)

	return model.ScoredAccount{
		Account:                     account,
		Classification:              class,
		CauseAlignmentScore:         alignment,
		EngagementAuthenticityScore: authenticity,
		ReceptivityScore:            receptivity,
		ReachScore:                  reach,
		CompositeScore:              round(composite),
		ConfidenceScore:             round(confidence),
		Recommendation:              recommendation,
		ConfidenceReasons:           confidenceReasons,
		Flags:                       slices.Compact(flags),
		Evidence:                    evidence,
	}
}

func computeRecommendation(alignment, authenticity, confidence float64, flags []string) (model.Recommendation, []string) {
	var reasons []string
	recommendation := model.RecommendationReview

	hasFlag := func(f string) bool {
		for _, flag := range flags {
			if flag == f {
				return true
			}
		}
		return false
	}

	if alignment >= 0.70 && authenticity >= 0.70 && confidence >= 0.65 {
		recommendation = model.RecommendationContact
		reasons = append(reasons, "Strong alignment, authentic engagement, and high confidence")
	} else if alignment >= 0.50 && authenticity >= 0.60 {
		recommendation = model.RecommendationReview
		reasons = append(reasons, "Moderate alignment and authenticity warrant manual review")
	} else if alignment < 0.40 || authenticity < 0.40 {
		recommendation = model.RecommendationAvoid
		reasons = append(reasons, "Low alignment or authenticity signals suggest avoiding outreach")
	}

	if hasFlag("hostile") || hasFlag("misaligned") {
		recommendation = model.RecommendationAvoid
		reasons = append(reasons, "Explicit hostile or misaligned signals detected")
	}

	if hasFlag("engagement_suspicious") || hasFlag("growth_suspicious") {
		recommendation = model.RecommendationReview
		reasons = append(reasons, "Suspicious engagement patterns require verification")
	}

	if hasFlag("opportunistic") {
		reasons = append(reasons, "Opportunistic behavior detected - verify before outreach")
	}

	if confidence < 0.50 {
		recommendation = model.RecommendationReview
		reasons = append(reasons, "Low confidence in data - manual verification needed")
	}

	if len(reasons) == 0 {
		reasons = append(reasons, "No explicit concerns detected")
	}

	return recommendation, reasons
}

func scoreAlignment(class model.Classification) (float64, []string, []string) {
	score := 0.40
	flags := []string{}

	switch class.AlignmentLabel {
	case "strong_animal_welfare":
		score = 0.95
	case "adjacent_progressive_cause":
		score = 0.72
	case "neutral_general_interest":
		score = 0.45
	case "commercial_only":
		score = 0.20
		flags = append(flags, "misaligned")
	case "misaligned_or_hostile":
		score = 0.05
		flags = append(flags, "misaligned")
	default:
		score = 0.30
		flags = append(flags, "manual_review_required")
	}

	if class.Opportunistic {
		score -= 0.10
		flags = append(flags, "opportunistic")
	}
	if class.Hostile {
		score = math.Min(score, 0.05)
		flags = append(flags, "hostile")
	}

	return clamp(score), class.Rationale, flags
}

func scoreAuthenticity(account model.Account) (float64, []string, []string) {
	score := 0.85
	var evidence []string
	var flags []string

	commentDiversity := uniqueCommentRatio(account.Comments)
	emojiHeavyRatio := emojiLikeCommentRatio(account.Comments)
	repetitiveRatio := repetitiveCommentRatio(account.Comments)
	growthSpike := followerGrowthSpike(account.GrowthSnapshots)

	timingAnomaly, timingEvidence := detectTimingAnomalies(account.Posts)
	commenterOverlap := detectCommenterOverlap(account.Comments, account.Posts)
	engagementRate := calculateEngagementRate(account)
	crossSignalMismatch := detectCrossSignalMismatch(account, engagementRate)

	if repetitiveRatio > 0.35 {
		score -= 0.25
		flags = append(flags, "engagement_suspicious")
		evidence = append(evidence, fmt.Sprintf("Repeated low-information comments detected across %.0f%% of sampled comments.", repetitiveRatio*100))
	}
	if emojiHeavyRatio > 0.45 {
		score -= 0.15
		flags = append(flags, "comment_quality_low")
		evidence = append(evidence, fmt.Sprintf("Emoji-only or generic praise comments are unusually frequent at %.0f%%.", emojiHeavyRatio*100))
	}
	if commentDiversity < 0.55 {
		score -= 0.15
		evidence = append(evidence, fmt.Sprintf("Comment text diversity is low at %.0f%% unique content.", commentDiversity*100))
	}
	if growthSpike > 0.30 {
		score -= 0.25
		flags = append(flags, "growth_suspicious")
		evidence = append(evidence, fmt.Sprintf("Follower growth spiked %.0f%% between snapshots without corroborating evidence.", growthSpike*100))
	}

	if timingAnomaly > 0 {
		score -= timingAnomaly * 0.15
		evidence = append(evidence, timingEvidence...)
		if timingAnomaly > 0.5 {
			flags = append(flags, "posting_pattern_suspicious")
		}
	}

	if commenterOverlap > 0.60 {
		score -= 0.20
		flags = append(flags, "commenter_repetition")
		evidence = append(evidence, fmt.Sprintf("High commenter overlap (%.0f%%) across posts detected.", commenterOverlap*100))
	}

	if crossSignalMismatch {
		score -= 0.25
		flags = append(flags, "cross_signal_mismatch")
		evidence = append(evidence, "Discrepancy between follower count and engagement rate suggests inflated metrics.")
	}

	expectedRate := expectedEngagementRate(account.Platform)
	if engagementRate < expectedRate*0.1 && account.FollowerCount > 10000 {
		score -= 0.20
		flags = append(flags, "engagement_suspicious")
		evidence = append(evidence, fmt.Sprintf("Engagement rate %.2f%% is below expected baseline %.2f%% for %s.", engagementRate*100, expectedRate*100, account.Platform))
	}

	if len(evidence) == 0 {
		evidence = append(evidence, "Comment and growth signals appear consistent with organic engagement.")
	}

	return clamp(score), evidence, flags
}

func detectTimingAnomalies(posts []model.ContentItem) (float64, []string) {
	if len(posts) < 3 {
		return 0, nil
	}

	hourCounts := map[int]int{}
	for _, post := range posts {
		hour := post.PostedAt.Hour()
		hourCounts[hour]++
	}

	suspiciousHours := 0
	for hour, count := range hourCounts {
		if (hour >= 0 && hour <= 5) && count > len(posts)/3 {
			suspiciousHours++
		}
	}

	if suspiciousHours > 0 {
		return 0.3, []string{"Unusual posting hours detected (bulk posts between midnight-5am)."}
	}

	return 0, nil
}

func detectCommenterOverlap(comments []model.CommentSample, posts []model.ContentItem) float64 {
	if len(comments) == 0 || len(posts) == 0 {
		return 0
	}

	postCommenters := map[string]map[string]struct{}{}
	for _, comment := range comments {
		if comment.ContentID == "" {
			continue
		}
		if postCommenters[comment.ContentID] == nil {
			postCommenters[comment.ContentID] = make(map[string]struct{})
		}
		postCommenters[comment.ContentID][comment.AuthorHandle] = struct{}{}
	}

	if len(postCommenters) < 2 {
		return 0
	}

	totalRepeatCommenters := 0
	uniqueCommenters := map[string]int{}
	for _, commenters := range postCommenters {
		for handle := range commenters {
			uniqueCommenters[handle]++
		}
	}

	for _, count := range uniqueCommenters {
		if count > 1 {
			totalRepeatCommenters++
		}
	}

	if len(uniqueCommenters) == 0 {
		return 0
	}

	return float64(totalRepeatCommenters) / float64(len(uniqueCommenters))
}

func calculateEngagementRate(account model.Account) float64 {
	if account.FollowerCount == 0 {
		return 0
	}
	totalEngagement := 0
	for _, post := range account.Posts {
		totalEngagement += post.LikeCount + post.CommentCount + post.ShareCount
	}
	if len(account.Posts) == 0 {
		return 0
	}
	avgPerPost := float64(totalEngagement) / float64(len(account.Posts))
	return avgPerPost / float64(account.FollowerCount)
}

func expectedEngagementRate(platform model.Platform) float64 {
	switch platform {
	case model.PlatformInstagram:
		return 0.05
	case model.PlatformX:
		return 0.02
	default:
		return 0.03
	}
}

func detectCrossSignalMismatch(account model.Account, engagementRate float64) bool {
	if account.FollowerCount > 50000 && account.PostCount > 100 {
		avgEngagementPerPost := (float64(account.PostCount) * engagementRate)
		if avgEngagementPerPost < 0.001 && engagementRate < 0.001 {
			return true
		}
	}

	avgEngagement := averageEngagement(account)
	if account.FollowerCount > 100000 && avgEngagement < 50 {
		return true
	}

	return false
}

func scoreReceptivity(class model.Classification) (float64, []string) {
	baseScore := 0.45
	evidence := []string{}

	baseScore += classifierInfluence(class)

	if class.ReceptivityScore > 0 {
		combinedScore := (0.40 * class.ReceptivityScore) + (0.60 * baseScore)
		evidence = append(evidence, fmt.Sprintf("Receptivity derived from classifier (%.2f) and independent features (%.2f).", class.ReceptivityScore, baseScore))
		return clamp(combinedScore), evidence
	}

	evidence = append(evidence, "Receptivity based on independent feature analysis.")
	return clamp(baseScore), evidence
}

func classifierInfluence(class model.Classification) float64 {
	switch class.ReceptivityLabel {
	case "high":
		return 0.35
	case "medium":
		return 0.10
	case "low":
		return -0.15
	case "very_low":
		return -0.30
	default:
		return 0.0
	}
}

// safeDivide returns numerator/denominator, falling back to fallback when the
// denominator is effectively zero.  This prevents NaN from propagating through
// the reach scoring pipeline.
func safeDivide(numerator, denominator, fallback float64) float64 {
	if math.Abs(denominator) < 1e-12 {
		return fallback
	}
	return numerator / denominator
}

func scoreReach(cfg config.Config, stats DatasetStats, account model.Account) (float64, []string) {
	percentiles := stats.FollowerPercentiles[account.Platform]
	engPercentiles := stats.EngagementPercentiles[account.Platform]

	logFollowers := math.Log1p(float64(account.FollowerCount))
	logAvgEngagement := math.Log1p(averageEngagement(account))

	var followerComponent float64
	if percentiles.P95 > 0 {
		logP50 := math.Log1p(percentiles.P50)
		logP90 := math.Log1p(percentiles.P90)
		logP95 := math.Log1p(percentiles.P95)

		switch {
		case logFollowers < logP50:
			followerComponent = 0.2 + 0.3*safeDivide(logFollowers, logP50, 0.5)
		case logP90 > logP50 && logFollowers < logP90:
			followerComponent = 0.5 + 0.3*safeDivide(logFollowers-logP50, logP90-logP50, 0.5)
		case logP95 > logP90 && logFollowers < logP95:
			followerComponent = 0.8 + 0.15*safeDivide(logFollowers-logP90, logP95-logP90, 0.5)
		default:
			followerComponent = 0.95
		}
	} else {
		followerComponent = 0.5
	}

	var engagementComponent float64
	if engPercentiles.P95 > 0 {
		logEP50 := math.Log1p(engPercentiles.P50)
		logEP90 := math.Log1p(engPercentiles.P90)
		logEP95 := math.Log1p(engPercentiles.P95)

		switch {
		case logAvgEngagement < logEP50:
			engagementComponent = 0.2 + 0.3*safeDivide(logAvgEngagement, logEP50, 0.5)
		case logEP90 > logEP50 && logAvgEngagement < logEP90:
			engagementComponent = 0.5 + 0.3*safeDivide(logAvgEngagement-logEP50, logEP90-logEP50, 0.5)
		case logEP95 > logEP90:
			engagementComponent = 0.8 + 0.2*safeDivide(logAvgEngagement-logEP90, logEP95-logEP90, 0.5)
		default:
			engagementComponent = 0.95
		}
	} else {
		engagementComponent = 0.5
	}

	multiplier := cfg.PlatformMultipliers[string(account.Platform)]
	if multiplier == 0 {
		multiplier = 1
	}

	score := ((0.65 * followerComponent) + (0.35 * engagementComponent)) * multiplier
	evidence := []string{fmt.Sprintf("Reach normalized within %s and weighted by platform multiplier %.2f.", account.Platform, multiplier)}
	return clamp(score), evidence
}

func scoreConfidence(account model.Account, class model.Classification) float64 {
	score := 0.25
	if len(account.Posts) >= 3 {
		score += 0.25
	}
	if len(account.Comments) >= 8 {
		score += 0.20
	}
	if len(account.GrowthSnapshots) >= 3 {
		score += 0.15
	}
	score += class.AlignmentConfidence * 0.15
	return clamp(score)
}

func averageEngagement(account model.Account) float64 {
	if len(account.Posts) == 0 {
		return 0
	}

	total := 0.0
	for _, post := range account.Posts {
		total += float64(post.LikeCount + post.CommentCount + post.ShareCount)
	}
	return total / float64(len(account.Posts))
}

func uniqueCommentRatio(comments []model.CommentSample) float64 {
	if len(comments) == 0 {
		return 0.5
	}

	unique := map[string]struct{}{}
	for _, comment := range comments {
		key := strings.TrimSpace(strings.ToLower(comment.Text))
		if key == "" {
			continue
		}
		unique[key] = struct{}{}
	}
	return float64(len(unique)) / float64(len(comments))
}

func repetitiveCommentRatio(comments []model.CommentSample) float64 {
	if len(comments) == 0 {
		return 0
	}

	counts := map[string]int{}
	for _, comment := range comments {
		key := strings.TrimSpace(strings.ToLower(comment.Text))
		if key == "" {
			continue
		}
		counts[key]++
	}

	repetitive := 0
	for _, count := range counts {
		if count > 1 {
			repetitive += count
		}
	}
	return float64(repetitive) / float64(len(comments))
}

func emojiLikeCommentRatio(comments []model.CommentSample) float64 {
	if len(comments) == 0 {
		return 0
	}

	lowInfo := 0
	for _, comment := range comments {
		text := strings.TrimSpace(strings.ToLower(comment.Text))
		switch text {
		case "nice", "wow", "love this", "great", "amazing", "🔥🔥🔥", "😍😍", "nice post":
			lowInfo++
			continue
		}
		if len([]rune(text)) <= 3 {
			lowInfo++
		}
	}
	return float64(lowInfo) / float64(len(comments))
}

func followerGrowthSpike(snapshots []model.GrowthSnapshot) float64 {
	if len(snapshots) < 2 {
		return 0
	}

	maxSpike := 0.0
	for i := 1; i < len(snapshots); i++ {
		prev := snapshots[i-1].FollowerCount
		curr := snapshots[i].FollowerCount
		if prev <= 0 {
			continue
		}
		growth := float64(curr-prev) / float64(prev)
		if growth > maxSpike {
			maxSpike = growth
		}
	}
	return maxSpike
}

func clamp(v float64) float64 {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return 0
	}
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return round(v)
}

func round(v float64) float64 {
	return math.Round(v*1000) / 1000
}
