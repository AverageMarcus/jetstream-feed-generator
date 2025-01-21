package consumer

import (
	"database/sql"
	dbpkg "jetstream-feed-generator/db/sqlc"
	"log/slog"
	"regexp"
	"strings"

	apibsky "github.com/bluesky-social/indigo/api/bsky"
	"github.com/bluesky-social/jetstream/pkg/models"
)

type ComposerErrorsFeed struct {
	name   string
	logger *slog.Logger
	q      *dbpkg.Queries
}

func NewComposerErrorsFeed(name string, logger *slog.Logger, db *sql.DB) *ComposerErrorsFeed {
	feedLogger := logger.With("feed", name)
	return &ComposerErrorsFeed{
		name:   name,
		logger: feedLogger,
		q:      dbpkg.New(db),
	}
}

func (f *ComposerErrorsFeed) Name() string {
	return f.name
}

func (f *ComposerErrorsFeed) DB() *dbpkg.Queries {
	return f.q
}

func (f *ComposerErrorsFeed) Match(event *models.Event, post *apibsky.FeedPost) bool {
	return isComposerError(post)
}

var domainRe = regexp.MustCompile("^https://(([A-Za-z0-9-]+)\\.([A-Za-z0-9]+))$")

func isComposerError(post *apibsky.FeedPost) bool {
	if post.Embed == nil || post.Embed.EmbedExternal == nil || post.Embed.EmbedExternal.External == nil {
		return false
	}
	uri := post.Embed.EmbedExternal.External.Uri
	matches := domainRe.FindStringSubmatch(uri)
	if len(matches) < 3 {
		return false
	}
	for _, facet := range post.Facets {
		for _, feature := range facet.Features {
			if feature.RichtextFacet_Link != nil && feature.RichtextFacet_Link.Uri == uri {
				return false
			}
		}
	}
	spaced := matches[2] + " " + matches[3]
	return strings.Contains(post.Text, spaced)
}
