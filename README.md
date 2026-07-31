# nzbget-exporter

Prometheus exporter for [NZBGet](https://nzbget.com/). Clean-room
implementation (MIT) that keeps the metric and environment contract of the
unlicensed `frebib/nzbget-exporter` image for the series this repo's owner
actually uses, so existing dashboards keep working.

Queries NZBGet's JSON-RPC API (`status`, `servervolumes`, `config`) on each
scrape.

## Metrics

| metric | type | meaning |
|---|---|---|
| `nzbget_up` | gauge | last API query succeeded (1/0) |
| `nzbget_thread_count` | gauge | threads in the NZBGet process |
| `nzbget_news_server_total_bytes{server}` | counter | bytes downloaded per news server (ID 0 aggregate excluded so `sum()` stays honest) |

## Configuration

| env | default | |
|---|---|---|
| `NZBGET_HOST` | — | base URL, e.g. `http://nzbget:6789` (required) |
| `NZBGET_USERNAME` | — | basic-auth username |
| `NZBGET_PASSWORD` | — | basic-auth password |
| `NZBGET_LISTEN` | `:9452` | listen address |

## Running

```
docker run -e NZBGET_HOST=http://nzbget:6789 -e NZBGET_USERNAME=... -e NZBGET_PASSWORD=... \
  -p 9452:9452 ghcr.io/jabrown93/nzbget-exporter:latest
```

Releases are cut by semantic-release from Conventional Commits; images are
multi-arch (amd64/arm64), cosign-signed, published to GHCR.
