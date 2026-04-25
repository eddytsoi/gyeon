-- ============================================================
-- Media foreign keys: link product images, post covers, and
-- category images to media_files records (Option 3).
-- Existing url/cover_image_url/image_url columns are kept as
-- Option 1/2 fallbacks.
-- ============================================================

ALTER TABLE product_images
    ADD COLUMN media_file_id UUID REFERENCES media_files(id) ON DELETE SET NULL,
    ALTER COLUMN url DROP NOT NULL;

ALTER TABLE cms_posts
    ADD COLUMN cover_media_file_id UUID REFERENCES media_files(id) ON DELETE SET NULL;

ALTER TABLE categories
    ADD COLUMN media_file_id UUID REFERENCES media_files(id) ON DELETE SET NULL;

CREATE INDEX idx_product_images_media_file_id ON product_images(media_file_id);
CREATE INDEX idx_cms_posts_cover_media_file_id ON cms_posts(cover_media_file_id);
CREATE INDEX idx_categories_media_file_id ON categories(media_file_id);
