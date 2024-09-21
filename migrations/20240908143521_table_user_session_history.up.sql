DROP TABLE IF EXISTS user_session_history;

CREATE TABLE IF NOT EXISTS
    public.user_session_history (
        session_id UUID PRIMARY KEY DEFAULT uuid_generate_v4 (),
        user_id UUID NOT NULL,
        auth_provider_id VARCHAR(255),
        login_time TIMESTAMP
        WITH
            TIME ZONE DEFAULT CURRENT_TIMESTAMP,
            logout_time TIMESTAMP
        WITH
            TIME ZONE DEFAULT NULL,
            ip_address VARCHAR,
            device_info VARCHAR,
            device_os VARCHAR,
            browser VARCHAR,
            user_agent TEXT,
            login_status VARCHAR,
            multifactor_enabled BOOLEAN,
            multifactor_verified BOOLEAN,
            -- Relaciones
            CONSTRAINT fk_history_user FOREIGN KEY (user_id) REFERENCES public.users (id) ON DELETE CASCADE
    );

-- Índice en la columna user_id
CREATE INDEX idx_user_session_history_user_id ON public.user_session_history (user_id);

-- Índice en la columna ip_address
CREATE INDEX idx_user_session_history_ip_address ON public.user_session_history (ip_address);

-- Índice en la columna login_time
CREATE INDEX idx_user_session_history_login_time ON public.user_session_history (login_time);