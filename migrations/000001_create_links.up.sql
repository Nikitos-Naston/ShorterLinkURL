CREATE TABLE IF NOT EXISTS links (
    code VARCHAR(10) COLLATE "C" PRIMARY KEY,
    original_url TEXT COLLATE "C" NOT NULL UNIQUE,

    CONSTRAINT links_code_format
        CHECK (code ~ '^[a-zA-Z0-9_]{10}$'),

    CONSTRAINT links_url_length
        CHECK (octet_length(original_url) BETWEEN 1 AND 2048)
);