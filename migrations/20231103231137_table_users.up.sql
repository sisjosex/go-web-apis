CREATE EXTENSION IF NOT EXISTS "uuid-ossp";

CREATE EXTENSION IF NOT EXISTS pgcrypto;

DROP TABLE IF EXISTS public.users;

CREATE TABLE IF NOT EXISTS
    public.users (
        id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
        first_name VARCHAR,
        last_name VARCHAR,
        phone VARCHAR,
        birthday DATE,
        email VARCHAR UNIQUE,
        password VARCHAR,
        is_active BOOLEAN DEFAULT TRUE,
        expiration_date TIMESTAMP
        WITH
            TIME ZONE DEFAULT NULL
    );

-- Índice en la columna is_active
CREATE INDEX idx_users_is_active ON public.users (is_active);

-- Índice en la columna email
CREATE INDEX idx_users_email ON public.users (email);

-- Índice en la columna birthday
CREATE INDEX idx_users_birthday ON public.users (birthday);

-- Índice en la columna expiration_date
CREATE INDEX idx_users_expiration_date ON public.users (expiration_date);