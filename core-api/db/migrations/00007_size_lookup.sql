-- Size normalisation (SPEC 4, "Size normalisation").
--
-- Maps (system, value) onto a single numeric scale per size_type, so a
-- child's current size can match items labelled in a different system
-- ("110", "4T" and "104-110" all land in the same range). Plain SQL, not a
-- Go migration: this is static numeric reference data with no branching
-- logic and no i18n, unlike 00006's category seed.
--
-- Not consumed by any query or endpoint in M2 — the table and its seed exist
-- now so M8 ("fits now"/"next size up") has something to read; M8 is the
-- first consumer. It carries no tenant data, so there is no family_id and no
-- sqlc query file yet.
--
-- The seed values are a deliberate simplification, not a sizing reference:
--   * clothing's "norm" scale is a height-in-cm continuum from infant to
--     adult. kids_cm uses the garment's own printed height label directly;
--     kids_eu approximates the age-in-years labelling some brands print
--     instead, mapped onto the nearest kids_cm bracket; intl_letter and
--     eu_adult are approximated onto the same continuum above child height,
--     which is good enough to say "this fits roughly here" but is not a
--     tailoring-grade conversion.
--   * shoe's "norm" scale is the EU size number. shoe_eu is seeded across
--     the full child-to-adult range (19-46), since EU is the label actually
--     printed on shoes bought in Ukraine. shoe_uk/shoe_us are seeded only
--     for the adult range (36-46): real UK sizing resets its own numbering
--     between child and adult shoes (a "UK 3.5" can be either), which this
--     single-row-per-value table cannot represent, and UK/US labels on
--     children's shoes are rare in this app's target market anyway.
--   * head currently only distinguishes one_size and a rough S/M/L band.
--
-- Partial coverage is an acceptable state (SPEC 4): an unmapped value simply
-- does not participate in fit matching.

-- +goose Up
CREATE TYPE size_type AS ENUM ('clothing', 'shoe', 'head');
CREATE TYPE size_system AS ENUM (
    'kids_cm', 'kids_eu', 'intl_letter', 'eu_adult',
    'shoe_eu', 'shoe_uk', 'shoe_us', 'one_size'
);

CREATE TABLE size_lookup (
    id        uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    system    size_system NOT NULL,
    size_type size_type   NOT NULL,
    value     text        NOT NULL,
    norm_min  numeric     NOT NULL,
    norm_max  numeric     NOT NULL,
    CONSTRAINT size_lookup_range_order CHECK (norm_min <= norm_max)
);

-- A label only has one meaning within a given system *and* size_type (the
-- same "S" means something different for clothing than for a hat).
CREATE UNIQUE INDEX size_lookup_system_type_value_idx
    ON size_lookup (system, size_type, value);

-- clothing: kids_cm — height in cm as printed on the garment.
INSERT INTO size_lookup (system, size_type, value, norm_min, norm_max) VALUES
    ('kids_cm', 'clothing', '50',  47,  53),
    ('kids_cm', 'clothing', '56',  53,  59),
    ('kids_cm', 'clothing', '62',  59,  65),
    ('kids_cm', 'clothing', '68',  65,  71),
    ('kids_cm', 'clothing', '74',  71,  77),
    ('kids_cm', 'clothing', '80',  77,  83),
    ('kids_cm', 'clothing', '86',  83,  89),
    ('kids_cm', 'clothing', '92',  89,  95),
    ('kids_cm', 'clothing', '98',  95, 101),
    ('kids_cm', 'clothing', '104', 101, 107),
    ('kids_cm', 'clothing', '110', 107, 113),
    ('kids_cm', 'clothing', '116', 113, 119),
    ('kids_cm', 'clothing', '122', 119, 125),
    ('kids_cm', 'clothing', '128', 125, 131),
    ('kids_cm', 'clothing', '134', 131, 137),
    ('kids_cm', 'clothing', '140', 137, 143),
    ('kids_cm', 'clothing', '146', 143, 149),
    ('kids_cm', 'clothing', '152', 149, 155),
    ('kids_cm', 'clothing', '158', 155, 161),
    ('kids_cm', 'clothing', '164', 161, 167),
    ('kids_cm', 'clothing', '170', 167, 173);

-- clothing: kids_eu — age-in-years labelling, approximated onto the kids_cm
-- bracket for a typical child of that age.
INSERT INTO size_lookup (system, size_type, value, norm_min, norm_max) VALUES
    ('kids_eu', 'clothing', '1',   77,  83),
    ('kids_eu', 'clothing', '2',   89,  95),
    ('kids_eu', 'clothing', '3',   95, 101),
    ('kids_eu', 'clothing', '4',  101, 107),
    ('kids_eu', 'clothing', '5',  107, 113),
    ('kids_eu', 'clothing', '6',  113, 119),
    ('kids_eu', 'clothing', '7',  119, 125),
    ('kids_eu', 'clothing', '8',  125, 131),
    ('kids_eu', 'clothing', '9',  131, 137),
    ('kids_eu', 'clothing', '10', 137, 143),
    ('kids_eu', 'clothing', '11', 143, 149),
    ('kids_eu', 'clothing', '12', 149, 155),
    ('kids_eu', 'clothing', '13', 155, 161),
    ('kids_eu', 'clothing', '14', 161, 167);

-- clothing: intl_letter and eu_adult, approximated onto the same continuum
-- above child height.
INSERT INTO size_lookup (system, size_type, value, norm_min, norm_max) VALUES
    ('intl_letter', 'clothing', 'XS',   150, 160),
    ('intl_letter', 'clothing', 'S',    160, 170),
    ('intl_letter', 'clothing', 'M',    170, 180),
    ('intl_letter', 'clothing', 'L',    180, 190),
    ('intl_letter', 'clothing', 'XL',   190, 200),
    ('intl_letter', 'clothing', 'XXL',  200, 210),
    ('intl_letter', 'clothing', 'XXXL', 210, 220),
    ('eu_adult', 'clothing', '32', 150, 155),
    ('eu_adult', 'clothing', '34', 155, 160),
    ('eu_adult', 'clothing', '36', 160, 165),
    ('eu_adult', 'clothing', '38', 165, 170),
    ('eu_adult', 'clothing', '40', 170, 175),
    ('eu_adult', 'clothing', '42', 175, 180),
    ('eu_adult', 'clothing', '44', 180, 185),
    ('eu_adult', 'clothing', '46', 185, 190),
    ('eu_adult', 'clothing', '48', 190, 195),
    ('eu_adult', 'clothing', '50', 195, 200),
    ('eu_adult', 'clothing', '52', 200, 205),
    ('eu_adult', 'clothing', '54', 205, 210),
    ('eu_adult', 'clothing', '56', 210, 215);

-- shoe: shoe_eu across the full child-to-adult range.
INSERT INTO size_lookup (system, size_type, value, norm_min, norm_max) VALUES
    ('shoe_eu', 'shoe', '19', 18.5, 19.5),
    ('shoe_eu', 'shoe', '20', 19.5, 20.5),
    ('shoe_eu', 'shoe', '21', 20.5, 21.5),
    ('shoe_eu', 'shoe', '22', 21.5, 22.5),
    ('shoe_eu', 'shoe', '23', 22.5, 23.5),
    ('shoe_eu', 'shoe', '24', 23.5, 24.5),
    ('shoe_eu', 'shoe', '25', 24.5, 25.5),
    ('shoe_eu', 'shoe', '26', 25.5, 26.5),
    ('shoe_eu', 'shoe', '27', 26.5, 27.5),
    ('shoe_eu', 'shoe', '28', 27.5, 28.5),
    ('shoe_eu', 'shoe', '29', 28.5, 29.5),
    ('shoe_eu', 'shoe', '30', 29.5, 30.5),
    ('shoe_eu', 'shoe', '31', 30.5, 31.5),
    ('shoe_eu', 'shoe', '32', 31.5, 32.5),
    ('shoe_eu', 'shoe', '33', 32.5, 33.5),
    ('shoe_eu', 'shoe', '34', 33.5, 34.5),
    ('shoe_eu', 'shoe', '35', 34.5, 35.5),
    ('shoe_eu', 'shoe', '36', 35.5, 36.5),
    ('shoe_eu', 'shoe', '37', 36.5, 37.5),
    ('shoe_eu', 'shoe', '38', 37.5, 38.5),
    ('shoe_eu', 'shoe', '39', 38.5, 39.5),
    ('shoe_eu', 'shoe', '40', 39.5, 40.5),
    ('shoe_eu', 'shoe', '41', 40.5, 41.5),
    ('shoe_eu', 'shoe', '42', 41.5, 42.5),
    ('shoe_eu', 'shoe', '43', 42.5, 43.5),
    ('shoe_eu', 'shoe', '44', 43.5, 44.5),
    ('shoe_eu', 'shoe', '45', 44.5, 45.5),
    ('shoe_eu', 'shoe', '46', 45.5, 46.5);

-- shoe: shoe_uk / shoe_us, adult range only (see header note).
INSERT INTO size_lookup (system, size_type, value, norm_min, norm_max) VALUES
    ('shoe_uk', 'shoe', '3.5',  35.5, 36.5),
    ('shoe_uk', 'shoe', '4',    36.5, 37.5),
    ('shoe_uk', 'shoe', '5',    37.5, 38.5),
    ('shoe_uk', 'shoe', '5.5',  38.5, 39.5),
    ('shoe_uk', 'shoe', '6.5',  39.5, 40.5),
    ('shoe_uk', 'shoe', '7',    40.5, 41.5),
    ('shoe_uk', 'shoe', '8',    41.5, 42.5),
    ('shoe_uk', 'shoe', '9',    42.5, 43.5),
    ('shoe_uk', 'shoe', '9.5',  43.5, 44.5),
    ('shoe_uk', 'shoe', '10.5', 44.5, 45.5),
    ('shoe_uk', 'shoe', '11',   45.5, 46.5),
    ('shoe_us', 'shoe', '4',    35.5, 36.5),
    ('shoe_us', 'shoe', '4.5',  36.5, 37.5),
    ('shoe_us', 'shoe', '5.5',  37.5, 38.5),
    ('shoe_us', 'shoe', '6',    38.5, 39.5),
    ('shoe_us', 'shoe', '7',    39.5, 40.5),
    ('shoe_us', 'shoe', '7.5',  40.5, 41.5),
    ('shoe_us', 'shoe', '8.5',  41.5, 42.5),
    ('shoe_us', 'shoe', '9.5',  42.5, 43.5),
    ('shoe_us', 'shoe', '10',   43.5, 44.5),
    ('shoe_us', 'shoe', '11',   44.5, 45.5),
    ('shoe_us', 'shoe', '11.5', 45.5, 46.5);

-- one_size — a sentinel that matches any requested size.
INSERT INTO size_lookup (system, size_type, value, norm_min, norm_max) VALUES
    ('one_size', 'clothing', 'one_size', 0, 999),
    ('one_size', 'shoe',     'one_size', 0, 999),
    ('one_size', 'head',     'one_size', 0, 999);

-- head: a rough circumference-based S/M/L band, in cm.
INSERT INTO size_lookup (system, size_type, value, norm_min, norm_max) VALUES
    ('intl_letter', 'head', 'S', 50, 54),
    ('intl_letter', 'head', 'M', 54, 58),
    ('intl_letter', 'head', 'L', 58, 62);

-- +goose Down
DROP TABLE IF EXISTS size_lookup;
DROP TYPE IF EXISTS size_system;
DROP TYPE IF EXISTS size_type;
