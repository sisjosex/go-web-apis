CREATE TABLE
    IF NOT EXISTS auth.user_profile (
        user_id UUID PRIMARY KEY REFERENCES auth.users (id),
        profile_picture_url TEXT,
        bio TEXT,
        website_url VARCHAR,
        -- otros campos del perfil
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );

CREATE INDEX idx_user_profile_created_at ON auth.user_profile (created_at);
CREATE INDEX idx_user_profile_updated_at ON auth.user_profile (updated_at);