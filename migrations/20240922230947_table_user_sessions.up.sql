DROP TABLE IF EXISTS public.user_sessions;

CREATE TABLE IF NOT EXISTS
    public.user_sessions (
        session_id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
        user_id UUID NOT NULL,
        provider_name VARCHAR(50) NOT NULL,  -- Nombre del proveedor (ej. 'email', 'facebook', 'google')
        auth_provider_id VARCHAR(255) NOT NULL,  -- ID específico del proveedor (como el ID de Facebook o Google)
        auth_token VARCHAR(255),  -- Token para el proveedor si aplica
        token_expiration TIMESTAMP WITH TIME ZONE,  -- Expiración del token
        login_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,  -- Tiempo de login
        logout_time TIMESTAMP WITH TIME ZONE DEFAULT NULL,  -- Tiempo de logout
        last_activity_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,  -- Última actividad
        ip_address INET,  -- Dirección IP del dispositivo
        device_os JSONB,  -- Información del dispositivo como sistema operativo, modelo, etc.
        user_agent VARCHAR(255),  -- User agent del dispositivo
        multifactor_enabled BOOLEAN,  -- Si MFA está activado
        multifactor_verified BOOLEAN,  -- Si MFA ha sido verificado
        is_active BOOLEAN DEFAULT TRUE,  -- Si la sesión está activa
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        -- Relaciones
        CONSTRAINT fk_user_session_user FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE
    );

CREATE INDEX idx_user_sessions_history_user_id ON public.user_sessions (user_id);
CREATE INDEX idx_user_sessions_provider_name ON public.user_sessions (provider_name);
CREATE INDEX idx_user_sessions_is_active ON public.user_sessions (is_active);
CREATE INDEX idx_user_sessions_created_at ON public.user_sessions (created_at);
CREATE INDEX idx_user_sessions_updated_at ON public.user_sessions (updated_at);