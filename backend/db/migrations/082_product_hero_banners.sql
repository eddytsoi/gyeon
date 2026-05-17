-- ============================================================
-- Per-product hero video + banner / media strip image slots
-- ============================================================
-- These 7 slots come from the WooCommerce ACF custom fields the
-- storefront uses for its product detail layout:
--   video    — YouTube ID, hero embed between specs grid and tabs
--   banner_1 — image displayed alongside the Description (content) tab
--   banner_2 — image displayed alongside the How-to-Use tab
--   media_1..media_4 — four images displayed above BundleComposer
--
-- Banner / media images go through the normal media_files pipeline
-- (WebP, resize-on-demand) so we just hold a foreign key. SET NULL
-- on delete: removing the underlying media_files row should not
-- cascade-delete the product, only blank the slot.
ALTER TABLE products
    ADD COLUMN video_id          VARCHAR(32),
    ADD COLUMN banner_1_media_id UUID REFERENCES media_files(id) ON DELETE SET NULL,
    ADD COLUMN banner_2_media_id UUID REFERENCES media_files(id) ON DELETE SET NULL,
    ADD COLUMN media_1_media_id  UUID REFERENCES media_files(id) ON DELETE SET NULL,
    ADD COLUMN media_2_media_id  UUID REFERENCES media_files(id) ON DELETE SET NULL,
    ADD COLUMN media_3_media_id  UUID REFERENCES media_files(id) ON DELETE SET NULL,
    ADD COLUMN media_4_media_id  UUID REFERENCES media_files(id) ON DELETE SET NULL;
