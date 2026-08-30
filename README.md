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

The builder renders every player and team page into the `public/` directory, copies static assets, emits a `search-index.json`, and packages the SQLite database at `public/downloads/oaamonitor.db`.

## Continuous deployment

GitHub Actions is wired to:

- Refresh the database on the cron schedule with `go run ./cmd/fetch -upload` and push it to `downloads/oaamonitor.db` in Bunny Storage (see `.github/workflows/refresh.yml`).
- Deploy the static site to Bunny Storage on every push to `main` after downloading and validating that same database object (see `.github/workflows/deploy.yml`).

The deployment and refresh workflows need the Bunny Storage zone password as
the `BUNNY_STORAGE_PASSWORD` repository secret. The bucket, New York S3
endpoint, region, and database key for `oaamonitor-benbailey-me-site` are
configured directly in the workflows.

For the first run after migrating, keep the previous object storage credentials
available as these repository secrets:

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_REGION`
- `AWS_ENDPOINT_URL_S3`

If Bunny does not yet contain `downloads/oaamonitor.db`, either workflow uses
those credentials once to bootstrap it from the previous `oaamonitor/oaamonitor.db`
object. They are no longer used after Bunny has the database and can then be
removed. Site deployment syncs `public/` to the zone and removes remote files
that are no longer in the generated site.
