-- Placeholder pricing from the PRD range: Rs 60 base and Rs 18 per km.
-- Phase 1 launches Attock City and Hasan Abdal. The other tehsils stay inactive.
INSERT INTO zones (
    id, city_name, slug, base_delivery_fee, per_km_rate, surge_multiplier, service_radius_km, is_active
) VALUES
    ('11111111-1111-4111-8111-111111111101', 'Attock City', 'attock-city', 60.00, 18.00, 1.00, 8.00, true),
    ('11111111-1111-4111-8111-111111111102', 'Hasan Abdal', 'hasan-abdal', 60.00, 18.00, 1.00, 8.00, true),
    ('11111111-1111-4111-8111-111111111103', 'Hazro', 'hazro', 60.00, 18.00, 1.00, 8.00, false),
    ('11111111-1111-4111-8111-111111111104', 'Fateh Jang', 'fateh-jang', 60.00, 18.00, 1.00, 8.00, false),
    ('11111111-1111-4111-8111-111111111105', 'Jand', 'jand', 60.00, 18.00, 1.00, 8.00, false),
    ('11111111-1111-4111-8111-111111111106', 'Pindi Gheb', 'pindi-gheb', 60.00, 18.00, 1.00, 8.00, false);
