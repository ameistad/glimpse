# Glimpse Project Instructions

## Project Structure

- Root Go module: `github.com/ameistad/glimpse`
- `assets/` - embedded CSS and local JS dependencies
- `templates/` - Go `html/template` files
- `docs/adr/` - architecture decision records

## Building for Linux

The app uses `go-sqlite3`, so CGO must be enabled. Cross-compiling from macOS requires `musl-cross`:

```bash
CGO_ENABLED=1 CC=x86_64-linux-musl-gcc GOOS=linux GOARCH=amd64 \
  go build -ldflags="-linkmode external -extldflags '-static'" \
  -o glimpse-linux .
```

Do not use plain `GOOS=linux go build`; it disables CGO and produces a broken SQLite build.

## Deploying to Server

Run the local deploy helper:

```bash
./deploy-storage.sh
```

The script runs tests, builds the Linux binary, copies it to `andreas@storage:~/glimpse-server`, and runs the remote deploy script with a TTY for sudo.

The deploy script (`/home/andreas/deploy-glimpse.sh`) handles:
- Stopping the systemd service
- Copying the binary to `/home/glimpse/`
- Resetting the database when needed
- Restarting the service

Note: the deploy script requires sudo, so run it interactively.

## Server Details

- Host: `andreas@storage`
- Service: `glimpse.service` (systemd)
- Binary location: `/home/glimpse/glimpse-server`
- Config: `/home/glimpse/config.json`
- Database: `/home/glimpse/glimpse.db`
- Thumbnails: `/home/glimpse/thumbnails/`
- Photos source: `/tank/andreas/Storage/Photos`

### Manual service control

```bash
sudo systemctl status glimpse
sudo systemctl stop glimpse
sudo systemctl start glimpse
sudo journalctl -u glimpse -f
```
