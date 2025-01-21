# jetstream-feed-generator

Glues together [Jetstream](https://github.com/bluesky-social/jetstream) and [go-bsky-feed-generator](https://github.com/ericvolp12/go-bsky-feed-generator/) with some SQLite to consume the Bluesky firehose and serve a feed based on posts matching some criteria.

Currently (11/24) in use serving [this feed](https://bsky.app/profile/roland.cros.by/feed/composer-errors), which detects when someone types a domain by accident, fixes it, and inadvertently leaves the link attachment.

---

## Example Config

```yaml
db:
  engine: sqlite
  connection_string: feeds.sqlite
feed_names:
- composer-errors
- english-text
log_level: INFO
log_format: json
consumer:
  enabled: true
  jetstream_url: wss://jetstream1.us-east.bsky.network/subscribe
  start_cursor: 0
feedgen:
  enabled: true
  port: 9072
  feed_actor_did: did:plc:replace-me-with-your-did
  service_endpoint: https://replace-me-with-your-service-endpoint.example.com
```

Run with:

```shell
go run . --config ./config.yml
```

The following URLs will then be available:

* http://localhost:9072/.well-known/did.json
* http://localhost:9072/xrpc/app.bsky.feed.describeFeedGenerator

Then for each feed enabled there will be an URL such as:
* http://localhost:9072/xrpc/app.bsky.feed.getFeedSkeleton?feed=at://did:plc:replace-me-with-your-did/app.bsky.feed.generator/english-text
