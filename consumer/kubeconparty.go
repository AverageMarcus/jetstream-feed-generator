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

type KubeConPartyFeed struct {
	name   string
	logger *slog.Logger
	q      *dbpkg.Queries
}

func NewKubeConPartyFeed(name string, logger *slog.Logger, db *sql.DB) *KubeConPartyFeed {
	feedLogger := logger.With("feed", name)
	return &KubeConPartyFeed{name, feedLogger, dbpkg.New(db)}
}

func (f *KubeConPartyFeed) Name() string {
	return f.name
}

func (f *KubeConPartyFeed) DB() *dbpkg.Queries {
	return f.q
}

func (f *KubeConPartyFeed) Match(event *models.Event, post *apibsky.FeedPost) bool {
	if len(post.Text) == 0 {
		return false
	}

	return f.isKubeConPost(post) && f.isPartyPost(post)
}

func (f *KubeConPartyFeed) isKubeConPost(post *apibsky.FeedPost) bool {
	var (
		// Official hashtags we want to include
		hashtags = []string{
			// KubeCon
			"KubeCon", "KubeConNA", "KubeConEU", "KubeConCN", "KubeConJP", "KubeConIN",
			"KubeConCloudNativeCon", "CloudNativeCon",
			// Experiences
			"KubeCrawl", "CloudNativeFest", "KubeCrawlCloudNativeFest",
		}
		// Some common phrases that might be used without hashtags that we'd want to match on
		re = regexp.MustCompile(`(?mi)\W(KubeCon|KubeConCloudNativeCon|CloudNativeCon)(EU|NA|JP|CN|IN)?(\d{2,4})?(\W|$)`)
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

func (f *KubeConPartyFeed) isPartyPost(post *apibsky.FeedPost) bool {
	var (
		// Official hashtags we want to include
		hashtags = []string{"KubeCrawl", "CloudNativeFest", "KubeCrawlCloudNativeFest", "KubeConParty"}
		// Some common phrases that might be used without hashtags that we'd want to match on
		re = regexp.MustCompile(`(?mi)\W(Party|Parties|Social|Meetup)(\W|$)`)
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
