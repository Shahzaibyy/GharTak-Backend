-- Enable Fateh Jang for local/demo testing (Attock district Phase-1 expansion).
UPDATE zones
SET is_active = true, updated_at = now()
WHERE slug = 'fateh-jang';
