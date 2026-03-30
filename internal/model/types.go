package model

import "time"

type Platform string

const (
	PlatformInstagram Platform = "instagram"
	PlatformX         Platform = "x"
)

type Account struct {
	AccountID       string           `json:"account_id"`
	Platform        Platform         `json:"platform"`
	Handle          string           `json:"handle"`
	DisplayName     string           `json:"display_name"`
	Bio             string           `json:"bio"`
	FollowerCount   int              `json:"follower_count"`
	FollowingCount  int              `json:"following_count"`
	PostCount       int              `json:"post_count"`
	Verified        bool             `json:"verified"`
	ExternalLinks   []string         `json:"external_links"`
	TopicsClaimed   []string         `json:"topics_claimed"`
	Posts           []ContentItem    `json:"posts"`
	Comments        []CommentSample  `json:"comments"`
	GrowthSnapshots []GrowthSnapshot `json:"growth_snapshots"`
}

type ContentItem struct {
	ContentID      string    `json:"content_id"`
	AccountID      string    `json:"account_id"`
	Platform       Platform  `json:"platform"`
	ContentType    string    `json:"content_type"`
	CaptionOrText  string    `json:"caption_or_text"`
	Hashtags       []string  `json:"hashtags"`
	PostedAt       time.Time `json:"posted_at"`
	LikeCount      int       `json:"like_count"`
	CommentCount   int       `json:"comment_count"`
	ShareCount     int       `json:"share_count"`
	ViewCount      int       `json:"view_count"`
	MediaLabels    []string  `json:"media_labels"`
	Pinned         bool      `json:"pinned"`
	HighEngagement bool      `json:"high_engagement"`
}

type CommentSample struct {
	CommentID    string    `json:"comment_id"`
	ContentID    string    `json:"content_id"`
	AuthorHandle string    `json:"author_handle"`
	Text         string    `json:"text"`
	PostedAt     time.Time `json:"posted_at"`
	IsReply      bool      `json:"is_reply"`
}

type GrowthSnapshot struct {
	CapturedAt    time.Time `json:"captured_at"`
	FollowerCount int       `json:"follower_count"`
}

type Classification struct {
	AlignmentLabel      string   `json:"alignment_label"`
	AlignmentConfidence float64  `json:"alignment_confidence"`
	ReceptivityLabel    string   `json:"receptivity_label"`
	ReceptivityScore    float64  `json:"receptivity_score"`
	Opportunistic       bool     `json:"opportunistic"`
	Hostile             bool     `json:"hostile"`
	Rationale           []string `json:"rationale"`
}

type Recommendation string

const (
	RecommendationContact Recommendation = "contact"
	RecommendationReview  Recommendation = "review"
	RecommendationAvoid   Recommendation = "avoid"
)

type ScoredAccount struct {
	Account                     Account        `json:"account"`
	Classification              Classification `json:"classification"`
	CauseAlignmentScore         float64        `json:"cause_alignment_score"`
	EngagementAuthenticityScore float64        `json:"engagement_authenticity_score"`
	ReceptivityScore            float64        `json:"receptivity_score"`
	ReachScore                  float64        `json:"reach_score"`
	CompositeScore              float64        `json:"composite_score"`
	ConfidenceScore             float64        `json:"confidence_score"`
	Recommendation              Recommendation `json:"recommendation"`
	ConfidenceReasons           []string       `json:"confidence_reasons"`
	Flags                       []string       `json:"flags"`
	Evidence                    []string       `json:"evidence"`
}

type Report struct {
	Topic   string          `json:"topic"`
	Results []ScoredAccount `json:"results"`
}
