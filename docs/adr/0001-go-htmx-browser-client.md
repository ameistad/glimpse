# Use a Go and HTMX browser client

Glimpse no longer maintains a separate Swift macOS client or JSON REST API; the supported client is a browser UI rendered by the Go app with HTMX and small Alpine.js interactions. This keeps browsing close to the server-side media model, removes the need to ship and configure a native client, and accepts a clean compatibility break for the old `/api/*` routes.
