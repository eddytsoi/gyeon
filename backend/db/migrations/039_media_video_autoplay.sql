-- ============================================================
-- Per-media autoplay flag for streaming video embeds
-- ============================================================
-- When TRUE, the storefront iframe is rendered with provider-specific
-- params for autoplay + mute + loop. Defaults FALSE so existing rows
-- behave the same as before.

ALTER TABLE media_files
    ADD COLUMN video_autoplay BOOLEAN NOT NULL DEFAULT FALSE;
