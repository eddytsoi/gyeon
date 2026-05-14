ALTER TABLE categories
    ADD COLUMN desktop_banner_url VARCHAR(1024),
    ADD COLUMN mobile_banner_url  VARCHAR(1024);

ALTER TABLE cms_post_categories
    ADD COLUMN desktop_banner_url VARCHAR(1024),
    ADD COLUMN mobile_banner_url  VARCHAR(1024);
