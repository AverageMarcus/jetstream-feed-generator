-- name: UpsertFeed :exec
insert into feeds (feed_name)
values ($1)
on conflict do nothing;

-- name: GetFeed :one
select *
from feeds
where feed_name = $1;

-- name: UpdateFeedCursor :exec
update feeds
set latest_cursor = $1
where feed_name = $2;

-- name: UpsertFeedPost :exec
insert
into feed_posts (feed_name, time_us, did, rkey)
values ($1, $2, $3, $4)
on conflict (feed_name, did, rkey) do nothing;

-- name: GetFeedPosts :many
select *
from feed_posts
where feed_name = $1
  and time_us < $2
order by time_us desc
limit $3;
