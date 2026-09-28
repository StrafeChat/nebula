# nebula

The object store behind a StrafeChat instance: avatars, space icons, custom emoji and
message attachments. One small Go service in front of pluggable storage — a local
directory or any S3-compatible bucket — so an instance can start on a disk and move to a
bucket without anything above it changing.

Part of [StrafeChat](https://github.com/StrafeChat). It is deployed for you by
[`StrafeChat/deploy`](https://github.com/StrafeChat/deploy); you only need the instructions
below if you are working on nebula itself.

## How it fits

`equinox` (the API) is the only thing that writes. A client uploading an avatar or an
attachment gets a signed URL from the API, which then `PUT`s the bytes here with the shared
`UPLOAD_SECRET`. Reads are public and unauthenticated — that is the point of a CDN — so
nothing private is ever stored unencrypted: end-to-end encrypted attachments arrive already
encrypted and nebula never sees a key.

## API

Everything lives under `/v1/<key>`, where the key is an ordinary path like
`avatars/<user id>/<hash>.webp`.

| Method | Auth | Behaviour |
| --- | --- | --- |
| `GET /v1/<key>` | none | The object, with `Cache-Control: max-age=$CACHE_CONTROL_MAX_AGE` and `Range` support for seeking in media. |
| `HEAD /v1/<key>` | none | Size and type without the body. |
| `PUT /v1/<key>` | `Authorization: Bearer $UPLOAD_SECRET` | Stores the body. Content type is inferred from the key's extension. |
| `DELETE /v1/<key>` | `Authorization: Bearer $UPLOAD_SECRET` | Removes it; 404 if it was not there. |
| `GET /health` | none | `{"ok":true,...}` plus which storage backend is live. |

The bearer check is constant-time, and with `UPLOAD_SECRET` unset both mutating methods are
refused — a read-only CDN, which is a reasonable way to run a replica.

## Bundled emoji

The web client renders Unicode emoji as images from an artwork set. Rather than pull those
from a third-party CDN - which would leak every viewer's IP and referrer and break on an
air-gapped instance - nebula ships the sets and serves them from this instance, under
`/v1/emoji/<set>/…` (so `<domain>/cdn/v1/emoji/twemoji/1f600.svg` behind Caddy).

`scripts/fetch-emoji.sh` downloads the pinned sets (twemoji, noto, openmoji) into
`EMOJI_ASSETS_DIR` at image-build time; on start, nebula copies any it does not already hold
into the store (`SEED_EMOJI`, default on). The copy is idempotent - it records the set
version and re-runs only when that changes - and runs in the background, so on a fresh store
emoji briefly 404 (the client falls back to the system font) until the first seed finishes.
A seed error is logged, never fatal. Building offline (`--build-arg FETCH_EMOJI=0`) or with
`SEED_EMOJI=false` simply serves no bundled emoji.

## Configuration

Environment variables only, no config file.

| Variable | Default | Notes |
| --- | --- | --- |
| `PORT` | `4010` | Distinct from equinox's 4000 and stargate's 4001. |
| `UPLOAD_SECRET` | *(unset)* | Shared with equinox. Unset ⇒ read-only. |
| `STORAGE_BACKEND` | `fs` | `fs` or `s3`. Anything else is a boot error. |
| `DATA_DIR` | `./data` | Where `fs` keeps objects (`/data` in Docker). |
| `HTTP_BODY_LIMIT_MB` | `25` | Upload ceiling; match equinox's attachment limit. |
| `CORS_ORIGINS` | *(empty)* | Comma-separated. Empty mirrors any `Origin` — fine locally, list your instances in production, because federated clients `fetch()` encrypted attachments cross-origin. |
| `CACHE_CONTROL_MAX_AGE` | `86400` | Seconds, on successful `GET`/`HEAD`. |
| `SEED_EMOJI` | `true` | Seed the bundled emoji sets into the store on start (see below). |
| `EMOJI_ASSETS_DIR` | `./emoji` | Where the bundled sets live (`/app/emoji` in Docker). |
| `S3_BUCKET` | — | Required when `STORAGE_BACKEND=s3`. |
| `S3_REGION` | `us-east-1` | |
| `S3_ENDPOINT` | *(empty)* | Empty means AWS; set it for MinIO, R2, B2, Ceph. |
| `S3_PREFIX` | *(empty)* | Optional key prefix inside the bucket. |
| `S3_ACCESS_KEY_ID`, `S3_SECRET_ACCESS_KEY` | — | Omit to use the ambient AWS credential chain. |
| `S3_FORCE_PATH_STYLE` | `false` | Required by MinIO and most self-hosted services. |

## Running it

```bash
cp .env.example .env     # then set UPLOAD_SECRET
go run ./cmd/nebula
```

`go test ./...` covers both storage backends; the S3 tests run against a stub, so no bucket
is needed.

## Layout

```
cmd/nebula        entry point
internal/config   environment parsing and validation
internal/server   Fiber app, routing, range requests, auth
internal/storage  the Storage interface plus the fs and s3 implementations
internal/middleware  CORS and security headers
```

Adding a backend means implementing `storage.Storage` (`Put`, `Get`, `Stat`, `Delete`,
`Name`) and one branch in `server.New` — nothing else knows which backend is in use.

## Licence

AGPL-3.0. See [LICENSE](LICENSE).

The bundled emoji artwork keeps its own upstream licences, shipped beside each set as
`emoji/<set>/LICENSE`: **Twemoji** graphics [CC-BY 4.0](https://creativecommons.org/licenses/by/4.0/),
**Noto Emoji** [Apache-2.0](https://www.apache.org/licenses/LICENSE-2.0), **OpenMoji**
[CC-BY-SA 4.0](https://creativecommons.org/licenses/by-sa/4.0/). The CC-BY / CC-BY-SA sets
require attribution, which the web client shows under Settings → Appearance → Emoji.
