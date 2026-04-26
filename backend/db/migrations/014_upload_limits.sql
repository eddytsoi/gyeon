INSERT INTO site_settings (key, value, description)
VALUES
  ('upload_max_image_mb', '1', 'Maximum image upload size in megabytes'),
  ('upload_max_video_mb', '10', 'Maximum video upload size in megabytes')
ON CONFLICT (key) DO NOTHING;
