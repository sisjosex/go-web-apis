DROP TABLE IF EXISTS auth_providers;

CREATE TABLE IF NOT EXISTS
    public.auth_providers (
        provider_id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
        provider_name VARCHAR(50) UNIQUE NOT NULL
    );

-- Insertar los proveedores comunes
INSERT INTO
    auth_providers (provider_name)
VALUES
    ('email'),
    ('facebook'),
    ('google'),
    ('hotmail');