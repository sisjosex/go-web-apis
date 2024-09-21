CREATE TABLE
    IF NOT EXISTS user_profile (
        user_id UUID PRIMARY KEY REFERENCES users (id),
        profile_picture_url TEXT,
        bio TEXT,
        website_url VARCHAR,
        -- otros campos del perfil
        created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
    );