# Glimpse

A fast RAW photo browser for your homelab. Glimpse generates lightweight JPEG thumbnails from your RAW photo library and serves them via a native macOS app, making it easy to browse large collections without the slowness of Samba file transfers.

## Architecture

```
┌─────────────────────────────────────────────────────────┐
│                 Debian Server (ZFS)                     │
│  ┌─────────────────────────────────────────────────┐   │
│  │  Go Service (glimpse-server)                     │   │
│  │  • Periodic directory traversal                  │   │
│  │  • RAW → JPEG thumbnail generation               │   │
│  │  • SQLite metadata storage                       │   │
│  │  • REST API for browsing + downloads             │   │
│  └─────────────────────────────────────────────────┘   │
│              │                                          │
│  /pool/photos/originals/   ←  Your RAW files           │
│  /pool/thumbnails/         ←  Generated JPEGs          │
└─────────────────────────────────────────────────────────┘
                          │ HTTP API
                          ▼
┌─────────────────────────────────────────────────────────┐
│              macOS App (Glimpse.app)                    │
│  • Native SwiftUI interface                             │
│  • Grid view with smooth scrolling                      │
│  • Download RAW files on demand                         │
└─────────────────────────────────────────────────────────┘
```

## Server Setup (Debian 12)

### Prerequisites

Install required packages:

```bash
sudo apt update
sudo apt install dcraw imagemagick
```

### ZFS Dataset Setup (Recommended)

Create a separate dataset for thumbnails:

```bash
# Create thumbnails dataset
sudo zfs create pool/thumbnails

# Optional: Set compression (thumbnails compress well)
sudo zfs set compression=lz4 pool/thumbnails

# Create the directory structure
sudo mkdir -p /pool/thumbnails
sudo chown $USER:$USER /pool/thumbnails
```

### Building the Server

On your Debian server (requires Go 1.21+):

```bash
# Clone or copy the server directory to your server
cd glimpse/server

# Build
go build -o glimpse-server .

# Or for a static binary
CGO_ENABLED=1 go build -ldflags="-s -w" -o glimpse-server .
```

### Configuration

Create a configuration file:

```bash
cp config.example.json config.json
```

Edit `config.json` to match your setup:

```json
{
  "originals_path": "/pool/photos/originals",
  "thumbnails_path": "/pool/thumbnails",
  "database_path": "/pool/thumbnails/glimpse.db",
  "listen_addr": ":8080",
  "scan_interval_seconds": 3600,
  "thumbnail_size": 800,
  "raw_extensions": [".cr2", ".cr3", ".nef", ".arw", ".dng", ".raf"]
}
```

| Option | Description |
|--------|-------------|
| `originals_path` | Path to your RAW photo directory |
| `thumbnails_path` | Where to store generated thumbnails (use separate ZFS dataset) |
| `database_path` | SQLite database location |
| `listen_addr` | HTTP server address (use `0.0.0.0:8080` to listen on all interfaces) |
| `scan_interval_seconds` | How often to scan for new photos (3600 = 1 hour) |
| `thumbnail_size` | Maximum dimension for thumbnails in pixels |
| `raw_extensions` | List of RAW file extensions to process |

### Running the Server

```bash
# Run directly
./glimpse-server -config config.json

# Or run in background with nohup
nohup ./glimpse-server -config config.json > glimpse.log 2>&1 &
```

### Systemd Service (Recommended)

Create `/etc/systemd/system/glimpse.service`:

```ini
[Unit]
Description=Glimpse Photo Server
After=network.target zfs-mount.service

[Service]
Type=simple
User=your-username
WorkingDirectory=/path/to/glimpse/server
ExecStart=/path/to/glimpse/server/glimpse-server -config /path/to/config.json
Restart=always
RestartSec=5

[Install]
WantedBy=multi-user.target
```

Enable and start:

```bash
sudo systemctl daemon-reload
sudo systemctl enable glimpse
sudo systemctl start glimpse
sudo systemctl status glimpse
```

### Firewall

If using ufw, allow access:

```bash
sudo ufw allow 8080/tcp
```

## macOS App Setup

### Building the App

On your Mac (requires Xcode Command Line Tools):

```bash
cd glimpse/app/Glimpse

# Build debug version
swift build

# Run directly
swift run

# Or build release version
swift build -c release
```

The built app will be in `.build/release/Glimpse` or `.build/debug/Glimpse`.

### Creating an App Bundle (Optional)

For a proper .app bundle, create an Xcode project:

1. Open Xcode → File → New → Project
2. Choose "App" under macOS
3. Name it "Glimpse", select SwiftUI
4. Delete the generated files and add the files from `Sources/`
5. Build and archive for distribution

### Configuration

1. Launch the app
2. Open Settings (Cmd+,)
3. Enter your server URL (e.g., `http://192.168.1.100:8080`)
4. Set your preferred download directory
5. Click "Test Connection" to verify

## API Reference

The server exposes a REST API:

| Endpoint | Description |
|----------|-------------|
| `GET /api/photos` | List photos (supports `folder`, `limit`, `offset` params) |
| `GET /api/photos/{id}` | Get photo metadata |
| `GET /api/photos/{id}/thumbnail` | Get thumbnail JPEG |
| `GET /api/photos/{id}/original` | Download original RAW file |
| `GET /api/folders` | List all folders with photo counts |
| `GET /api/stats` | Get library statistics |

## Supported RAW Formats

- Canon: `.cr2`, `.cr3`
- Nikon: `.nef`, `.nrw`
- Sony: `.arw`, `.srf`, `.sr2`
- Olympus: `.orf`
- Pentax: `.pef`
- Fuji: `.raf`
- Panasonic: `.rw2`
- Adobe: `.dng`
- Leica: `.rwl`
- Hasselblad: `.3fr`, `.fff`
- Phase One: `.iiq`

## Troubleshooting

### Thumbnails not generating

1. Check dcraw is installed: `dcraw -v`
2. Check ImageMagick is installed: `convert -version`
3. Check server logs: `journalctl -u glimpse -f`
4. Verify file permissions on originals and thumbnails directories

### App can't connect to server

1. Verify server is running: `curl http://your-server:8080/api/stats`
2. Check firewall settings
3. Ensure server is listening on the correct interface (`0.0.0.0` for all)

### Slow thumbnail generation

The first scan takes time as it processes all RAW files. Subsequent scans only process new/modified files. Consider:

- Using `-h` flag in dcraw for half-size extraction (already enabled)
- Processing during off-hours via the scan interval setting

## License

MIT
