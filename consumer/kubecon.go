package consumer

import (
	"database/sql"
	dbpkg "jetstream-feed-generator/db/sqlc"
	"log/slog"
	"regexp"
	"slices"
	"strings"
	"time"

	apibsky "github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/jetstream/pkg/models"
)

type KubeConFeed struct {
	name   string
	logger *slog.Logger
	q      *dbpkg.Queries
}

func NewKubeConFeed(name string, logger *slog.Logger, db *sql.DB) *KubeConFeed {
	feedLogger := logger.With("feed", name)
	return &KubeConFeed{name, feedLogger, dbpkg.New(db)}
}

func (f *KubeConFeed) Name() string {
	return f.name
}

func (f *KubeConFeed) DB() *dbpkg.Queries {
	return f.q
}

func (f *KubeConFeed) Match(event *models.Event, post *apibsky.FeedPost) bool {
	if post.Embed != nil || post.Reply != nil || len(post.Text) == 0 {
		return false
	}

	var (
		// Official hashtags we want to include
		hashtags = []string{
			// KubeCon
			"KubeCon", "KubeConNA", "KubeConEU", "KubeConCN", "KubeConJP", "KubeConIN",
			"KubeConCloudNativeCon", "CloudNativeCon",
			// Experiences
			"KubeCrawl", "CloudNativeFest", "KubeCrawlCloudNativeFest",
			"ContribFest",
			// Maintainer Summit
			"CNMaintainerSummit", "KubernetesMaintainerSummit", "KubeConMaintainerSummit",
			// Co-Located Events
			"ArgoCon", "BackstageCon", "CiliumCon", "CNK8sAIDay", "CNTelcoDay", "CloudNativeUniversity", "dokday",
			"EnvoyCon", "istioday", "KFSummit", "K8sEdgeDay", "LinkerdDay", "ObservabilityDay", "OpenFeature",
			"OpenTofuDay", "PlatEngDay",
			// Related Events
			"Rejekts", "RejektsEU", "RejektsNA",
		}
		// Some common phrases that might be used without hashtags that we'd want to match on
		re = regexp.MustCompile(`(?mi)\W(KubeCon|KubeConCloudNativeCon|CloudNativeCon|Rejekts)(EU|NA|JP|CN|IN)?(\d{2,4})?(\W|$)`)
	)

	// Used to include year-specific hashtags for each
	yearFull := time.Now().Format("2006")
	yearShort := time.Now().Format("06")

	// First check for relevant hashtags matching exactly (case-insensitive)
	for _, facet := range post.Facets {
		for _, feat := range facet.Features {
			if feat.RichtextFacet_Tag != nil {
				hashtag := feat.RichtextFacet_Tag.Tag
				if slices.ContainsFunc(hashtags, func(h string) bool {
					return strings.EqualFold(h, hashtag) || strings.EqualFold(h, hashtag+yearFull) || strings.EqualFold(h, hashtag+yearShort)
				}) {
					return true
				}
			}
		}
	}

	// Finally attempt some more generic non-hashtag matches
	return re.MatchString(post.Text)
}
