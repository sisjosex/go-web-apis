CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE EXTENSION IF NOT EXISTS pgcrypto;

CREATE SCHEMA IF NOT EXISTS auth;

DROP TABLE IF EXISTS auth.users;

CREATE TABLE IF NOT EXISTS
    auth.users (
        id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4 (),
        first_name VARCHAR(100),
        last_name VARCHAR(100),
        phone VARCHAR(15),
        birthday DATE,
        email VARCHAR(255) DEFAULT NULL,
        password VARCHAR(255) DEFAULT NULL,  -- Contraseña en formato binario (hash)
        is_active BOOLEAN DEFAULT TRUE,
        email_verified BOOLEAN DEFAULT TRUE,
        expiration_date TIMESTAMP WITH TIME ZONE DEFAULT NULL,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );

CREATE INDEX idx_users_is_active ON auth.users (is_active);
CREATE INDEX idx_users_email_verified ON auth.users (email_verified);
CREATE INDEX idx_users_email ON auth.users (email);
CREATE INDEX idx_users_birthday ON auth.users (birthday);
CREATE INDEX idx_users_expiration_date ON auth.users (expiration_date);
CREATE INDEX idx_users_created_at ON auth.users (created_at);
CREATE INDEX idx_users_updated_at ON auth.users (updated_at);

-- Index for compare lower email
CREATE INDEX idx_users_lower_email ON auth.users (LOWER(email));

CREATE TABLE IF NOT EXISTS auth.email_verification_tokens (
    id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4 (),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token UUID UNIQUE NOT NULL DEFAULT public.uuid_generate_v4 (),
    new_email VARCHAR NOT NULL,
    expires_at TIMESTAMP NOT NULL,
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_email_verify_user_id ON auth.email_verification_tokens (user_id);
CREATE INDEX idx_email_verify_token ON auth.email_verification_tokens (token);
CREATE INDEX idx_email_verify_new_email ON auth.email_verification_tokens (new_email);
CREATE INDEX idx_email_verify_expires_at ON auth.email_verification_tokens (expires_at);


CREATE TABLE IF NOT EXISTS auth.password_reset_tokens (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token TEXT NOT NULL,
    expires_at TIMESTAMP NOT NULL DEFAULT (NOW() + INTERVAL '1 hour'),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_forgot_user_id ON auth.password_reset_tokens (user_id);
CREATE INDEX idx_forgot_token ON auth.password_reset_tokens (token);
CREATE INDEX idx_forgot_expires_at ON auth.password_reset_tokens (expires_at);
