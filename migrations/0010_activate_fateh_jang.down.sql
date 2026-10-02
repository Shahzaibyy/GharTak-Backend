UPDATE zones
SET is_active = false, updated_at = now()
WHERE slug = 'fateh-jang';
