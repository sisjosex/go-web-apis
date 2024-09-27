CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE EXTENSION IF NOT EXISTS pgcrypto;

DROP TABLE IF EXISTS public.users;

CREATE TABLE IF NOT EXISTS
    public.users (
        id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
        first_name VARCHAR(50),
        last_name VARCHAR(50),
        phone VARCHAR(15),
        birthday DATE,
        email VARCHAR UNIQUE NOT NULL,
        password VARCHAR(255),  -- Contraseña en formato binario (hash)
        is_active BOOLEAN DEFAULT TRUE,
        is_verified BOOLEAN DEFAULT TRUE,
        expiration_date TIMESTAMP WITH TIME ZONE DEFAULT NULL,
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
    );

CREATE INDEX idx_users_is_active ON public.users (is_active);
CREATE INDEX idx_users_is_verified ON public.users (is_verified);
CREATE INDEX idx_users_email ON public.users (email);
CREATE INDEX idx_users_birthday ON public.users (birthday);
CREATE INDEX idx_users_expiration_date ON public.users (expiration_date);
CREATE INDEX idx_users_created_at ON public.users (created_at);
CREATE INDEX idx_users_updated_at ON public.users (updated_at);