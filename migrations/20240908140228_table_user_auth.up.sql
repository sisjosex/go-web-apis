DROP TABLE IF EXISTS user_auth;

CREATE TABLE IF NOT EXISTS
  public.user_auth (
    auth_id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
    user_id UUID NOT NULL,
    provider_id UUID NOT NULL,
    auth_provider_id VARCHAR(255) NOT NULL,
    auth_token VARCHAR(255),
    token_expiration TIMESTAMP WITH TIME ZONE,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    -- Relaciones
    CONSTRAINT fk_user FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE,
    CONSTRAINT fk_provider FOREIGN KEY (provider_id) REFERENCES public.auth_providers (provider_id) ON DELETE CASCADE
  );

-- Índice en la columna user_id
CREATE INDEX idx_user_auth_user_id ON public.user_auth (user_id);

-- Índice en la columna provider_id
CREATE INDEX idx_user_auth_provider_id ON public.user_auth (provider_id);

-- Índice en la columna auth_provider_id
CREATE INDEX idx_user_auth_auth_provider_id ON public.user_auth (auth_provider_id);

-- Índice en la columna user_id, provider_id, auth_provider_id
CREATE INDEX idx_user_auth_user_provider ON public.user_auth (user_id, provider_id, auth_provider_id);