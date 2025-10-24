# OAA Monitor

The Go code which powers the [Outs Above Average Monitor](https://oaamonitor.benbailey.me) website.

## Refresh data

Download the latest Baseball Savant snapshot and write it into `data/oaamonitor.db`:

```bash
go run ./cmd/fetch -database data/oaamonitor.db
```

Pass `-from-storage` to pull the most recently uploaded SQLite file from object storage, or `-upload` to push the refreshed database (credentials are taken from the usual environment variables).

## Static site generation

Once the SQLite database is up to date, render the static bundle (HTML, JSON, assets) to the target folder:

```bash
go run ./cmd/build -database data/oaamonitor.db -out public
```

The builder renders every player and team page into the `public/` directory, copies static assets, emits a `search-index.json`, and packages the SQLite database at `public/downloads/oaamonitor.db`.

## Continuous deployment

GitHub Actions is wired to:

- Refresh the database on the cron schedule with `go run ./cmd/fetch -upload` and push it to S3-compatible storage (see `.github/workflows/refresh.yml`).
- Deploy the static site to GitHub Pages on every push to `main` after downloading the latest database from storage (see `.github/workflows/deploy.yml`).

Set these repository secrets so both workflows can authenticate against your storage bucket:

- `AWS_ACCESS_KEY_ID`
- `AWS_SECRET_ACCESS_KEY`
- `AWS_REGION`
- `AWS_ENDPOINT_URL_S3` (optional; defaults to Fly/Tigris)

GitHub Pages handles the hosting; once the `deploy` workflow runs, the published site is available under the repository’s Pages URL (or any custom domain configured in the repo settings).
