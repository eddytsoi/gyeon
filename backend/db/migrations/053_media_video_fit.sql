-- ============================================================
-- Per-media fit mode for video / streaming-video embeds
-- ============================================================
-- Controls how the player renders inside its parent container on the
-- storefront: 'contain' letterboxes to preserve aspect ratio, 'cover'
-- fills the parent and crops overflow. Defaults 'contain' so existing
-- rows behave the same as before.

ALTER TABLE media_files
    ADD COLUMN video_fit TEXT NOT NULL DEFAULT 'contain'
        CHECK (video_fit IN ('contain', 'cover'));
