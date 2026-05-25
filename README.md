# Glimpse

A fast browser-based media library for a homelab photo archive. Glimpse scans originals, generates lightweight JPEG thumbnails, stores metadata in SQLite, and serves a Go + HTMX web UI for browsing, previewing, streaming supported videos, and downloading originals.

## Architecture

```
Debian/ZFS host
├── Go web app (`glimpse`)
│   ├── periodic media scan
│   ├── thumbnail generation
│   ├── SQLite metadata store
│   └── HTML/HTMX browser UI
├── originals_path      source media archive
└── thumbnails_path     generated JPEG thumbnails
```

The browser UI is the supported client. The old Swift macOS client and JSON `/api/*` routes have been removed.

## Setup

Install required system packages:

```bash
sudo apt update
sudo apt install dcraw imagemagick ffmpeg
```

Build the app:

```bash
go build -o glimpse .
```

Create a config file:

```bash
cp config.example.json config.json
```

Edit `config.json`:

```json
{
  "originals_path": "/pool/photos/originals",
  "thumbnails_path": "/pool/thumbnails",
  "database_path": "/pool/thumbnails/glimpse.db",
  "listen_addr": ":8080",
  "api_key": "",
  "scan_interval_seconds": 3600,
  "thumbnail_size": 800
}
```

Run it:

```bash
./glimpse -config config.json
```

Open `http://your-server:8080/media`. If `api_key` is set, the web UI shows a login form and stores an HttpOnly browser session cookie after a successful login.

## Configuration

| Option | Description |
|--------|-------------|
| `originals_path` | Path to the original media archive |
| `thumbnails_path` | Where generated thumbnails are stored |
| `database_path` | SQLite database path |
| `listen_addr` | HTTP listen address; use `0.0.0.0:8080` for LAN access |
| `api_key` | Optional browser login key |
| `scan_interval_seconds` | How often to scan for new or changed media |
| `thumbnail_size` | Maximum thumbnail dimension in pixels |
| `raw_extensions` | Still-image extensions to process |
| `video_extensions` | Video extensions to process |

## Systemd

Example `/etc/systemd/system/glimpse.service`:

```ini
[Unit]
Description=Glimpse Media Library
After=network.target zfs-mount.service

[Service]
Type=simple
User=glimpse
WorkingDirectory=/home/glimpse
ExecStart=/home/glimpse/glimpse -config /home/glimpse/config.json
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

## Database Reset

This version uses a `media_items` SQLite schema. If Glimpse finds the old `photos` schema, startup fails with an instruction to delete the database and rescan. This is intentional; existing deployments already treat the database as rebuildable from originals and thumbnails.

## Supported Formats

Still images include common RAW formats plus JPEG and PNG. Videos include `.mp4`, `.mov`, `.mkv`, `.avi`, `.webm`, `.m4v`, `.wmv`, and `.flv`. Browser playback depends on the browser's native video support; unsupported video formats remain downloadable.

## Troubleshooting

- Thumbnail generation needs `dcraw` and ImageMagick `convert`.
- Video thumbnails and metadata need `ffmpeg` and `ffprobe`.
- The first scan can take a long time; later scans skip unchanged media.
- If the browser redirects to login unexpectedly, confirm `api_key` in `config.json` and clear the `glimpse_session` cookie.

## Development

```bash
go test ./...
go build -o glimpse .
```

Run the local development server with hot reload:

```bash
./dev.sh
```

`dev.sh` builds a development binary, restarts it when Go files, templates, assets, `go.mod`, `go.sum`, or `.dev/config.json` change, and enables a dev-only browser reload hook. The dev server disables long-lived asset caching so CSS and JavaScript changes are picked up after each reload.

The web UI uses embedded local assets from `assets/` and templates from `templates/`.
