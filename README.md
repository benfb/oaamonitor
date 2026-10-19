# OAA Monitor

The Go code which powers the [Outs Above Average Monitor](https://oaamonitor.benbailey.me) website.

## Refresh data

Download the latest Baseball Savant snapshot into `data/oaamonitor.db` (created if missing). The
file is checkpointed and integrity-checked afterwards:

```bash
go run ./cmd/fetch -database data/oaamonitor.db
```

The database lives in object storage at `downloads/oaamonitor.db`. Use the `aws` CLI to move it,
for example `aws s3 cp s3://oaamonitor-benbailey-me-site/downloads/oaamonitor.db data/oaamonitor.db
--endpoint-url https://ny-s3.storage.bunnycdn.com`.

## Static site generation

Once the SQLite database is up to date, render the static bundle (HTML, JSON, assets) to the target folder:

```bash
go run ./cmd/build -database data/oaamonitor.db -out public
```

Templates and static assets are embedded in the binary, so the builder can run from any directory.
It renders every player and team page, copies the minified static assets, and emits a
`search-index.json`.

## Continuous deployment

A single workflow, `.github/workflows/site.yml`, runs on every push to `main` and on the refresh
schedule (08:00, 14:00, 20:00 and 02:00 UTC). Each run:

1. runs the tests and downloads the database from Bunny Storage;
2. on scheduled or manual runs, refreshes it, then updates the weekly copy (`backups/YYYY-Www-oaamonitor.db`) followed by
   the database itself;
3. builds the site, syncs `public/` to the zone, and purges the pull-zone cache.

Runs are serialized, so the database is never refreshed and deployed concurrently. The sync excludes
`downloads/` and `backups/`, which hold the live database and its weekly backups in the same zone;
without those excludes `--delete` removes the database. A year of weekly backups is about 1.5 GB and is never pruned automatically. Each run overwrites
that week's copy, so a bad refresh is only recoverable from the previous week's file.

The workflow needs the Bunny Storage zone password as the `BUNNY_STORAGE_PASSWORD` repository
secret. The bucket, New York S3 endpoint, region, and database key for
`oaamonitor-benbailey-me-site` are configured at the top of the workflow.

Cache purging also requires:

- `BUNNY_API_KEY` as a repository secret, using the account API key from Bunny's Account page
- `BUNNY_PULL_ZONE_ID` as a repository variable, using the numeric ID of the pull zone connected to the storage zone
