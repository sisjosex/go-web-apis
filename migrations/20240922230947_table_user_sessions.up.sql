DROP TABLE IF EXISTS auth.user_sessions;

CREATE TABLE IF NOT EXISTS
    auth.user_sessions (
        session_id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4 (),
        user_id UUID NOT NULL,
        provider_name VARCHAR(50) NOT NULL,  -- Nombre del proveedor (ej. 'email', 'facebook', 'google')
        auth_provider_id VARCHAR(255) NOT NULL,  -- ID específico del proveedor (como el ID de Facebook o Google)
        login_time TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,  -- Tiempo de login
        logout_time TIMESTAMP WITH TIME ZONE DEFAULT NULL,  -- Tiempo de logout
        ip_address VARCHAR(255),  -- Dirección IP del dispositivo
        device_id UUID, -- ID del dispositivo
        device_info VARCHAR(255), -- Dispositivo (ej. iPhone 12)
        browser VARCHAR(255),  -- Navegador del dispositivo
        device_os VARCHAR(255),  -- Información del dispositivo como sistema operativo, modelo, etc.
        user_agent TEXT,  -- User agent del dispositivo
        multifactor_enabled BOOLEAN,  -- TODO: Si MFA está activado
        multifactor_verified BOOLEAN,  -- TODO: Si MFA ha sido verificado
        is_active BOOLEAN DEFAULT TRUE,  -- Si la sesión está activa
        created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
        -- Relaciones
        CONSTRAINT fk_user_session_user FOREIGN KEY (user_id) REFERENCES auth.users (id) ON DELETE CASCADE
    );

CREATE INDEX idx_user_sessions_created_at ON auth.user_sessions (created_at);
CREATE INDEX idx_user_sessions_updated_at ON auth.user_sessions (updated_at);

CREATE INDEX idx_user_sessions_verify ON auth.user_sessions (
    user_id, provider_name, auth_provider_id, device_id, is_active
);
