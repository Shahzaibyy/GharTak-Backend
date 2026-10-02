-- Zone centers for Flutter camera lock and service-area checks.
ALTER TABLE zones
    ADD COLUMN center_lat DOUBLE PRECISION,
    ADD COLUMN center_lng DOUBLE PRECISION;

UPDATE zones SET center_lat = 33.7667, center_lng = 72.3667 WHERE slug = 'attock-city';
UPDATE zones SET center_lat = 33.8194, center_lng = 72.6892 WHERE slug = 'hasan-abdal';
UPDATE zones SET center_lat = 33.9092, center_lng = 72.4919 WHERE slug = 'hazro';
UPDATE zones SET center_lat = 33.5672, center_lng = 72.6417 WHERE slug = 'fateh-jang';
UPDATE zones SET center_lat = 33.4297, center_lng = 72.0181 WHERE slug = 'jand';
UPDATE zones SET center_lat = 33.2417, center_lng = 72.2667 WHERE slug = 'pindi-gheb';

ALTER TABLE zones
    ALTER COLUMN center_lat SET NOT NULL,
    ALTER COLUMN center_lng SET NOT NULL;
