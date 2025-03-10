package consumer

import (
	"context"
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"

	dbpkg "jetstream-feed-generator/db/sqlc"

	apibsky "github.com/bluesky-social/indigo/api/bsky"
	jetstreamClient "github.com/bluesky-social/jetstream/pkg/client"
	"github.com/bluesky-social/jetstream/pkg/client/schedulers/sequential"
	"github.com/bluesky-social/jetstream/pkg/models"
)

const DefaultJetstreamURL = "wss://jetstream1.us-east.bsky.network/subscribe"

type Config struct {
	JetstreamURL string
	StartCursor  int64
	DB           *sql.DB
	Stats        bool
	FeedNames    []string
}

type Feed interface {
	Name() string
	DB() *dbpkg.Queries
	Match(event *models.Event, post *apibsky.FeedPost) bool
}

func RunConsumer(ctx context.Context, config Config) error {
	logger := slog.With("component", "consumer")

	getFeed := func(name string) *Feed {
		var feed Feed
		switch name {
		case "composer-errors":
			feed = NewComposerErrorsFeed("composer-errors", logger, config.DB)
		case "english-text":
			feed = NewEnglishTextFeed("english-text", logger, config.DB)
		case "kubecon":
			feed = NewKubeConFeed("kubecon", logger, config.DB)
		case "kubecon-party":
			feed = NewKubeConPartyFeed("kubecon-party", logger, config.DB)
		}
		return &feed
	}

	enabledFeeds := []Feed{}
	for _, f := range config.FeedNames {
		feed := getFeed(f)
		if feed != nil {
			enabledFeeds = append(enabledFeeds, *feed)
		}
	}

	handler := handler{
		feeds:        enabledFeeds,
		latestCursor: config.StartCursor,
	}

	for _, f := range handler.feeds {
		if err := f.DB().UpsertFeed(ctx, f.Name()); err != nil {
			return fmt.Errorf("failed to initialize feed %s: %v", f.Name(), err)
		}
	}

	var lag float64
	if handler.latestCursor != 0 {
		lag = time.Since(time.UnixMicro(handler.latestCursor)).Seconds()
		logger.Info("starting at requested cursor", "cursor", handler.latestCursor, "lag_s", lag)
	} else {
		var resumeCursor int64
		for _, f := range handler.feeds {
			var err error
			feedCursor := int64(0)
			feed, err := f.DB().GetFeed(ctx, f.Name())
			if err == nil && feed.LatestCursor.Valid {
				feedCursor = feed.LatestCursor.Int64
			}

			if err != nil {
				return fmt.Errorf("failed to get latest cursor for feed %s: %v", f.Name(), err)
			}
			if feedCursor != 0 && (resumeCursor == 0 || feedCursor < resumeCursor) {
				resumeCursor = feedCursor
			}
		}

		if resumeCursor == 0 {
			handler.latestCursor = time.Now().UnixMicro()
			logger.Info("no saved cursor in database, starting at current time", "cursor", handler.latestCursor)
		} else {
			lag = time.Since(time.UnixMicro(resumeCursor)).Seconds()
			logger.Info("resuming from saved cursor", "saved_cursor", resumeCursor, "lag_s", lag)
			handler.latestCursor = resumeCursor
		}
	}
	lag = time.Since(time.UnixMicro(handler.latestCursor)).Seconds()
	logger.Info("starting consumer", "cursor", handler.latestCursor, "lag_s", lag)

	jetstreamConfig := jetstreamClient.DefaultClientConfig()
	jetstreamConfig.WebsocketURL = config.JetstreamURL
	jetstreamConfig.Compress = true
	jetstreamConfig.WantedCollections = append(jetstreamConfig.WantedCollections, "app.bsky.feed.post")

	scheduler := sequential.NewScheduler("jetstream-feed-generator", logger, handler.HandleEvent)

	c, err := jetstreamClient.NewClient(jetstreamConfig, logger, scheduler)
	if err != nil {
		return fmt.Errorf("failed to create Jetstream client: %v", err)
	}

	// Every 5 seconds print stats and update the high-water mark in the DB
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		for {
			select {
			case t := <-ticker.C:
				for _, f := range handler.feeds {
					err := f.DB().UpdateFeedCursor(ctx, dbpkg.UpdateFeedCursorParams{
						LatestCursor: sql.NullInt64{Int64: handler.latestCursor, Valid: true},
						FeedName:     f.Name(),
					})
					if err != nil {
						logger.Error("failed to save cursor", "feed", f.Name(), "error", err)
					}
				}
				if t.Second()%5 == 0 && config.Stats {
					eventsRead := c.EventsRead.Load()
					bytesRead := c.BytesRead.Load()
					avgEventSize := bytesRead / eventsRead
					lag := time.Now().Sub(time.UnixMicro(handler.latestCursor)).Seconds()
					logger.Info(
						"stats", "events_read", eventsRead, "bytes_read", bytesRead,
						"avg_event_size", avgEventSize, "latest_cursor", handler.latestCursor, "lag_s", lag,
					)
				}
			case <-ctx.Done():
				logger.Info("shutdown", "latest_cursor", handler.latestCursor)
				return
			}
		}
	}()

	for {
		if err := c.ConnectAndRead(ctx, &handler.latestCursor); err != nil {
			if strings.Contains(err.Error(), "unexpected EOF") || strings.Contains(err.Error(), "bad handshake") {
				logger.Error(
					"Failed to read from websocket, we're going to skip to the next cursor...",
					"latest_cursor", handler.latestCursor,
					"err", err,
				)
				handler.latestCursor = handler.latestCursor + 1
				continue
			} else if !strings.HasPrefix(err.Error(), "read loop failed") {
				return fmt.Errorf("failed to connect: %v", err)
			}
			// Lets retry...
			logger.Warn("Failed to read from websocket, we're going to retry...")
		}
	}
}

type handler struct {
	feeds        []Feed
	latestCursor int64
}

func (h *handler) HandleEvent(ctx context.Context, event *models.Event) error {
	logger := slog.With("component", "consumer")
	if event.Commit != nil && (event.Commit.Operation == models.CommitOperationCreate || event.Commit.Operation == models.CommitOperationUpdate) {
		switch event.Commit.Collection {
		case "app.bsky.feed.post":
			var post apibsky.FeedPost
			if err := json.Unmarshal(event.Commit.Record, &post); err != nil {
				logger.Error(
					"failed to unmarshal post",
					"record", event.Commit.Record,
					"did", event.Did,
					"error", err,
				)
				break
			}
			for _, f := range h.feeds {
				if f.Match(event, &post) {
					logger.Info(
						"post matched", "feed", f.Name(),
						"did", event.Did, "rkey", event.Commit.RKey, "text", post.Text,
					)
					err := f.DB().UpsertFeedPost(ctx, dbpkg.UpsertFeedPostParams{
						FeedName: f.Name(),
						TimeUs:   event.TimeUS,
						Did:      event.Did,
						Rkey:     event.Commit.RKey,
					})
					if err != nil {
						return fmt.Errorf("failed to upsert feed post: %w", err)
					}
				}
			}
		}
	}

	h.latestCursor = event.TimeUS

	return nil
}
