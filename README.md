# OpenPaws — Influencer Credibility Engine

A CLI-based scoring and ranking system that evaluates social media accounts for cause alignment, engagement authenticity, outreach receptivity, and reach. Built for advocacy organizations to identify credible outreach targets and flag inauthentic or hostile accounts.

## Architecture

```
openpaws/
├── cmd/cred-engine/         # CLI entrypoint — argument parsing, I/O
│   └── main.go
├── internal/
│   ├── cli/                 # Console formatting, platform parsing
│   │   └── cli.go
│   ├── config/              # Scoring weights, platform multipliers, LLM config
│   │   └── config.go
│   ├── connector/           # Data ingestion from fixture files
│   │   └── fixture.go
│   ├── llm/                 # LLM provider interface + mock + OpenAI implementation
│   │   ├── provider.go      # Provider interface, OpenAI client, validation, retry logic
│   │   └── mock.go          # Deterministic keyword-based classifier for offline use
│   ├── model/               # Canonical data types shared across all packages
│   │   └── types.go
│   ├── pipeline/            # Orchestration — loads data, classifies, scores, ranks
│   │   └── rank.go
│   └── scoring/             # All scoring logic — alignment, authenticity, reach, receptivity
│       └── scorer.go
├── testdata/fixtures/       # Fixture dataset (5 accounts across Instagram + X)
│   └── accounts.json
├── reports/                 # Generated JSON reports
│   └── sample.json          # Reference output
├── go.mod
└── plan.md                  # Original product specification
```

### Design Principles

| Principle | Implementation |
|---|---|
| **Separation of concerns** | Ingestion, classification, and scoring live in distinct packages with no circular dependencies |
| **Provider abstraction** | `llm.Provider` interface decouples scoring from any specific LLM — swap `mock` ↔ `openai` via a CLI flag |
| **Deterministic scoring** | All scoring after LLM classification is rule-based and reproducible |
| **Platform normalization** | Reach is normalized within each platform via log-percentile scaling before cross-platform ranking |
| **Configurable weights** | All composite weights and platform multipliers are defined in `config.Config`, not hardcoded |

---

## Data Model

Defined in [`internal/model/types.go`](internal/model/types.go). All types use JSON struct tags for direct serialization.

### Core Types

| Type | Purpose | Key Fields |
|---|---|---|
| `Account` | Canonical representation of a social media account | `account_id`, `platform`, `handle`, `bio`, `follower_count`, `posts`, `comments`, `growth_snapshots` |
| `ContentItem` | A single post/reel/tweet | `content_id`, `caption_or_text`, `hashtags`, `like_count`, `comment_count`, `share_count`, `posted_at`, `pinned`, `high_engagement` |
| `CommentSample` | A sampled comment on a post | `comment_id`, `content_id`, `author_handle`, `text`, `is_reply` |
| `GrowthSnapshot` | Point-in-time follower count | `captured_at`, `follower_count` |
| `Classification` | LLM output — alignment + receptivity assessment | `alignment_label`, `alignment_confidence`, `receptivity_label`, `receptivity_score`, `opportunistic`, `hostile`, `rationale` |
| `ScoredAccount` | Final scored result with all dimensions | `composite_score`, `cause_alignment_score`, `engagement_authenticity_score`, `receptivity_score`, `reach_score`, `confidence_score`, `flags`, `evidence`, `recommendation` |
| `Report` | Top-level output container | `topic`, `results[]` |

### Alignment Taxonomy

The LLM classifies each account into one of six labels:

| Label | Meaning |
|---|---|
| `strong_animal_welfare` | Deep, sustained advocacy for animal welfare causes |
| `adjacent_progressive_cause` | Related causes (climate, sustainability, ethical food) without direct focus |
| `neutral_general_interest` | No clear cause alignment in either direction |
| `commercial_only` | Primarily commercial/sponsored content with weak or opportunistic cause signals |
| `misaligned_or_hostile` | Actively opposes or attacks the campaign topic |
| `uncertain` | Insufficient data for confident classification |

---

## Scoring Pipeline

The pipeline runs in [`internal/pipeline/rank.go`](internal/pipeline/rank.go) and executes four stages sequentially:

```
Fixture Ingestion → LLM Classification → Multi-Dimensional Scoring → Composite Ranking
```

### Stage 1: Ingestion

[`internal/connector/fixture.go`](internal/connector/fixture.go) reads all `.json` files from the input directory, unmarshals them as `[]Account`, and filters by requested platforms.

### Stage 2: Classification

The [`llm.Provider`](internal/llm/provider.go) interface classifies each account:

```go
type Provider interface {
    ClassifyAccount(ctx context.Context, topic string, account model.Account) (model.Classification, error)
}
```

**Two implementations:**

- **`MockProvider`** — Deterministic keyword-counting classifier. Counts term hits across bio + topics + posts to assign labels. Zero external dependencies. Used for testing and demo.
- **`OpenAIProvider`** — Sends structured prompts to the OpenAI API with `response_format: json_object`. Includes retry with exponential backoff (100ms → 500ms → 2s) and output validation against the alignment taxonomy. Falls back to `uncertain` default on exhausted retries.

### Stage 3: Scoring

All scoring logic lives in [`internal/scoring/scorer.go`](internal/scoring/scorer.go). Each dimension is computed independently.

#### Cause Alignment Score (weight: `0.40`)

Maps the LLM alignment label to a base score:

| Label | Base Score |
|---|---|
| `strong_animal_welfare` | 0.95 |
| `adjacent_progressive_cause` | 0.72 |
| `neutral_general_interest` | 0.45 |
| `commercial_only` | 0.20 |
| `misaligned_or_hostile` | 0.05 |

Modifiers: `opportunistic` → −0.10, `hostile` → capped at 0.05.

#### Engagement Authenticity Score (weight: `0.25`)

Rule-based analysis of seven signals:

| Signal | Detection Method | Penalty |
|---|---|---|
| **Repetitive comments** | Same normalized text appearing >35% of comments | −0.25, flag `engagement_suspicious` |
| **Emoji/generic praise** | Short generic comments ("nice", "wow", "🔥🔥🔥") >45% | −0.15, flag `comment_quality_low` |
| **Low comment diversity** | Unique comment texts <55% | −0.15 |
| **Follower growth spike** | >30% growth between consecutive snapshots | −0.25, flag `growth_suspicious` |
| **Posting hour anomalies** | >⅓ of posts between midnight–5am | −0.15 × severity |
| **Commenter overlap** | Same users commenting across >60% of posts | −0.20, flag `commenter_repetition` |
| **Cross-signal mismatch** | High followers but engagement rate <0.1% of platform baseline | −0.25, flag `cross_signal_mismatch` |
| **Low engagement rate** | Engagement rate <10% of platform expected rate with >10k followers | −0.20, flag `engagement_suspicious` |

Base score starts at 0.85. Penalties stack and the result is clamped to `[0, 1]`.

#### Receptivity Score (weight: `0.20`)

Combines the LLM's receptivity signal with independent feature analysis:

```
combined = 0.40 × classifier_receptivity + 0.60 × feature_score
```

The feature score starts at 0.45 and is adjusted by the classifier's receptivity label (`high` → +0.35, `medium` → +0.10, `low` → −0.15, `very_low` → −0.30).

#### Reach Score (weight: `0.15`)

Uses **log-percentile scaling** to normalize within each platform:

1. Compute log-transformed follower count and average engagement
2. Map to percentile bands (P50, P75, P90, P95) within the platform dataset
3. Blend: `0.65 × follower_component + 0.35 × engagement_component`
4. Apply platform multiplier (Instagram: `1.0`, X/Twitter: `0.8`)

Division-by-zero guards (`safeDivide`) prevent NaN when percentile boundaries collapse on small datasets.

#### Confidence Score

Not part of the composite weight — used as a meta-signal:

| Condition | Contribution |
|---|---|
| Base | 0.25 |
| ≥3 posts | +0.25 |
| ≥8 comments | +0.20 |
| ≥3 growth snapshots | +0.15 |
| LLM alignment confidence | +0.15 × confidence |

Accounts below `LowConfidenceFloor` (default `0.45`) get a 10% composite penalty and `low_confidence` + `manual_review_required` flags.

### Stage 4: Ranking

```
composite = (alignment × 0.40) + (authenticity × 0.25) + (receptivity × 0.20) + (reach × 0.15)
```

Results are sorted descending by composite score, with confidence as tiebreaker.

### Recommendations

Each account receives an outreach recommendation:

| Recommendation | Condition |
|---|---|
| `contact` | Alignment ≥ 0.70, authenticity ≥ 0.70, confidence ≥ 0.65 |
| `review` | Moderate signals, suspicious flags, or low confidence |
| `avoid` | Alignment < 0.40, hostile/misaligned flags |

---

## Configuration

All tunable parameters in [`internal/config/config.go`](internal/config/config.go):

```go
config.Default() → Config{
    Weights: {
        CauseAlignment: 0.40,
        Authenticity:   0.25,
        Receptivity:    0.20,
        Reach:          0.15,
    },
    PlatformMultipliers: {
        "instagram": 1.0,
        "x":         0.8,
    },
    LowConfidenceFloor: 0.45,
    OpenAI: {
        BaseURL: "https://api.openai.com/v1",
        Model:   "gpt-4.1-mini",
    },
}
```

---

## Usage

### Prerequisites

- Go 1.23+
- (Optional) `OPENAI_API_KEY` environment variable for live LLM classification

### Build

```bash
go build -o cred-engine ./cmd/cred-engine
```

### Run with Mock Provider (no API key needed)

```bash
./cred-engine rank \
  --topic "ban factory farming" \
  --input ./testdata/fixtures \
  --platforms instagram,x \
  --output ./reports/results.json \
  --provider mock
```

### Run with OpenAI Provider

```bash
export OPENAI_API_KEY="sk-..."
./cred-engine rank \
  --topic "ban factory farming" \
  --input ./testdata/fixtures \
  --platforms instagram,x \
  --output ./reports/results.json \
  --provider openai
```

### CLI Flags

| Flag | Required | Default | Description |
|---|---|---|---|
| `--topic` | Yes | — | Campaign topic to evaluate against |
| `--input` | Yes | — | Directory containing fixture JSON files |
| `--output` | No | — | Path to write JSON report |
| `--platforms` | No | `instagram,x` | Comma-separated platform filter |
| `--provider` | No | `openai` | LLM provider: `openai` or `mock` |

### Example Output

```
Campaign Topic: ban factory farming
Accounts Ranked: 5

1. @rescuevoices (instagram)
   Composite: 0.897 | Alignment: 0.950 | Authenticity: 0.850 | Receptivity: 0.808 | Reach: 0.950

2. @ethicalfoodnow (x)
   Composite: 0.868 | Alignment: 0.950 | Authenticity: 0.850 | Receptivity: 0.808 | Reach: 0.760

3. @lifestylemax (instagram)
   Composite: 0.374 | Alignment: 0.100 | Authenticity: 0.700 | Receptivity: 0.292 | Reach: 0.950
   Flags: comment_quality_low, low_confidence, manual_review_required, misaligned, opportunistic

4. @viralpetbuzz (x)
   Composite: 0.337 | Alignment: 0.450 | Authenticity: 0.000 | Receptivity: 0.320 | Reach: 0.617
   Flags: comment_quality_low, engagement_suspicious, growth_suspicious

5. @antiveganvoice (x)
   Composite: 0.300 | Alignment: 0.050 | Authenticity: 0.850 | Receptivity: 0.110 | Reach: 0.523
   Flags: hostile, low_confidence, manual_review_required, misaligned
```

---

## Test Fixtures

The fixture dataset in [`testdata/fixtures/accounts.json`](testdata/fixtures/accounts.json) contains five accounts designed to exercise all scoring paths:

| Account | Platform | Role | Expected Behavior |
|---|---|---|---|
| `@rescuevoices` | Instagram | Strong animal welfare advocate | Ranks #1 — high alignment, organic engagement, strong confidence |
| `@ethicalfoodnow` | X | Adjacent cause (climate/sustainability) | Ranks #2 — strong alignment via adjacent cause content |
| `@lifestylemax` | Instagram | Commercial/opportunistic (180k followers) | Flagged `misaligned`, `opportunistic`, `comment_quality_low` — high reach but weak alignment |
| `@viralpetbuzz` | X | Inauthentic (210k followers, bot comments, 188% growth spike) | Flagged `engagement_suspicious`, `growth_suspicious` — authenticity score drops to 0.00 |
| `@antiveganvoice` | X | Hostile/misaligned | Flagged `hostile`, `misaligned` — alignment capped at 0.05 |

---

## Testing

```bash
go test ./... -v -count=1
```

### Test Coverage

| Package | Tests | What's Covered |
|---|---|---|
| `internal/pipeline` | 2 | End-to-end ranking: aligned account ranks first, inauthentic account flagged, hostile account penalized |
| `internal/scoring` | 18 | Alignment scoring (all labels), authenticity (organic vs bot), reach (NaN edge cases with small datasets), confidence (sparse vs rich data), composite weighting, recommendations, clamp guards, growth spike detection, comment quality ratios |
| `internal/llm` | 8 | Mock provider (all 4 classification paths), validation (valid labels, missing label, invalid label, out-of-range confidence) |
| `internal/connector` | 5 | Fixture loading, platform filtering, missing directory, empty directory, invalid JSON |
| `internal/cli` | 3 | Platform string parsing, console report formatting, empty report |

### Key Assertions

- A genuinely aligned account (`@rescuevoices`) **always** ranks above a larger but weakly aligned account (`@lifestylemax` at 180k followers)
- The inauthentic account (`@viralpetbuzz`) is **always** flagged with `engagement_suspicious` or `growth_suspicious`
- The hostile account (`@antiveganvoice`) receives a cause alignment score ≤ 0.10 and carries the `hostile` flag
- Reach scoring **never produces NaN**, even with 1–2 account datasets where percentile boundaries collapse

---

## Supported Platforms

| Platform | Multiplier | Status |
|---|---|---|
| Instagram | 1.0 | ✅ Supported |
| X (Twitter) | 0.8 | ✅ Supported |
| YouTube | — | Planned (connector interface ready) |
| TikTok | — | Planned (connector interface ready) |

Adding a new platform requires:
1. Adding a `Platform` constant in `model/types.go`
2. Adding a multiplier entry in `config.go`
3. Providing fixture data in the expected `[]Account` JSON format
4. Updating platform parsing in `cli/cli.go`

---

## JSON Report Schema

The `--output` flag writes a full JSON report:

```json
{
  "topic": "ban factory farming",
  "results": [
    {
      "account": { /* full Account object */ },
      "classification": {
        "alignment_label": "strong_animal_welfare",
        "alignment_confidence": 0.90,
        "receptivity_label": "high",
        "receptivity_score": 0.82,
        "opportunistic": false,
        "hostile": false,
        "rationale": ["..."]
      },
      "cause_alignment_score": 0.95,
      "engagement_authenticity_score": 0.85,
      "receptivity_score": 0.808,
      "reach_score": 0.95,
      "composite_score": 0.897,
      "confidence_score": 0.985,
      "recommendation": "contact",
      "confidence_reasons": ["Strong alignment, authentic engagement, and high confidence"],
      "flags": [],
      "evidence": ["..."]
    }
  ]
}
```

---

## License

See repository for license details.
