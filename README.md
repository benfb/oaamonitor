# OAA Monitor

The Go code which powers the [Outs Above Average Monitor](https://oaamonitor.benbailey.me) website.

## Refresh data

Download the latest Baseball Savant snapshot and write it into `data/oaamonitor.db`:

```bash
go run ./cmd/fetch -database data/oaamonitor.db
```

Pass `-from-storage` to pull the most recently uploaded SQLite file from object storage, or `-upload` to push the refreshed database (credentials are taken from the usual environment variables). Set `DATABASE_STORAGE_BUCKET` and `DATABASE_STORAGE_KEY` to override the default `oaamonitor/oaamonitor.db` object location.

## Static site generation

Once the SQLite database is up to date, render the static bundle (HTML, JSON, assets) to the target folder:

```bash
go run ./cmd/build -database data/oaamonitor.db -out public
```

The builder renders every player and team page into the `public/` directory, copies static assets (minified builds only), and emits a `search-index.json`.

## Continuous deployment

GitHub Actions is wired to:

- Refresh the database on the cron schedule with `go run ./cmd/fetch -upload` and push it to `downloads/oaamonitor.db` in Bunny Storage (see `.github/workflows/refresh.yml`).
- Deploy the static site to Bunny Storage on every push to `main`, then purge the connected Bunny CDN pull zone after downloading and validating that same database object (see `.github/workflows/deploy.yml`).

The deployment and refresh workflows need the Bunny Storage zone password as
the `BUNNY_STORAGE_PASSWORD` repository secret. The bucket, New York S3
endpoint, region, and database key for `oaamonitor-benbailey-me-site` are
configured directly in the workflows.

Cache purging also requires:

- `BUNNY_API_KEY` as a repository secret, using the account API key from Bunny's Account page
- `BUNNY_PULL_ZONE_ID` as a repository variable, using the numeric ID of the pull zone connected to the storage zone

Site deployment syncs `public/` to the zone and removes remote files that are
no longer in the generated site. The sync excludes `downloads/` and `backups/`,
which hold the live database and its dated backups in the same zone; without
those excludes `--delete` removes the database. A successful sync is followed by a full
pull-zone cache purge so changed pages are served immediately.
