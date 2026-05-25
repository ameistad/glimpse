# Require explicit database reset for the media items schema

The server now uses a `media_items` schema instead of the old broad `photos` schema. Rather than silently migrate or drop data on startup, Glimpse fails with a clear reset instruction when it detects the old schema, because the SQLite database is rebuildable from the originals and this avoids surprising destructive behavior.
