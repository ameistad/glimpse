# Glimpse Project Instructions

## Project Structure

- `Glimpse/` - macOS SwiftUI app
- `server/` - Go backend server

## Building the Server for Linux

The server uses `go-sqlite3` which requires CGO. Cross-compiling from macOS requires `musl-cross`:

```bash
cd server
CGO_ENABLED=1 CC=x86_64-linux-musl-gcc GOOS=linux GOARCH=amd64 \
  go build -ldflags="-linkmode external -extldflags '-static'" \
  -o glimpse-server-linux .
```

Do NOT use plain `GOOS=linux go build` as it disables CGO and produces a broken binary.

## Deploying to Server

1. Build the Linux binary (see above)

2. Copy to server:
   ```bash
   scp server/glimpse-server-linux andreas@storage:~/glimpse-server
   ```

3. SSH to server and run the deploy script:
   ```bash
   ssh andreas@storage
   ./deploy-glimpse.sh
   ```

The deploy script (`/home/andreas/deploy-glimpse.sh`) handles:
- Stopping the systemd service
- Copying the binary to `/home/glimpse/`
- Resetting the database (removes glimpse.db for fresh scan)
- Restarting the service

Note: The deploy script requires sudo, so run it interactively.

## Server Details

- Host: `andreas@storage`
- Service: `glimpse.service` (systemd)
- Binary location: `/home/glimpse/glimpse-server`
- Config: `/home/glimpse/config.json`
- Database: `/home/glimpse/glimpse.db`
- Thumbnails: `/home/glimpse/thumbnails/`
- Photos source: `/tank/andreas/Storage/Photos`

### Manual service control:
```bash
sudo systemctl status glimpse
sudo systemctl stop glimpse
sudo systemctl start glimpse
sudo journalctl -u glimpse -f  # follow logs
```
