--
-- PostgreSQL database dump
--


SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET transaction_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

--
-- Name: auth; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA auth;


--
-- Name: email_verification_tokens; Type: TABLE; Schema: auth; Owner: -
--

CREATE TABLE auth.email_verification_tokens (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    token uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    new_email character varying,
    expires_at timestamp without time zone NOT NULL,
    created_at timestamp without time zone DEFAULT now()
);


--
-- Name: otp_requests; Type: TABLE; Schema: auth; Owner: -
--

CREATE TABLE auth.otp_requests (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    destination character varying(255) NOT NULL,
    otp_channel character varying(50) NOT NULL,
    otp_code character varying(6) NOT NULL,
    user_id uuid,
    is_verified boolean DEFAULT false,
    verified_at timestamp with time zone,
    attempts integer DEFAULT 0,
    max_attempts integer DEFAULT 5,
    invalidated_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    expires_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: password_reset_tokens; Type: TABLE; Schema: auth; Owner: -
--

CREATE TABLE auth.password_reset_tokens (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid NOT NULL,
    token text NOT NULL,
    expires_at timestamp without time zone DEFAULT (now() + '01:00:00'::interval) NOT NULL,
    created_at timestamp without time zone DEFAULT now()
);


--
-- Name: push_devices; Type: TABLE; Schema: auth; Owner: -
--

CREATE TABLE auth.push_devices (
    user_id uuid NOT NULL,
    device_id character varying(100) NOT NULL,
    platform character varying(10) NOT NULL,
    token text NOT NULL,
    last_seen_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_push_devices_platform CHECK (((platform)::text = ANY ((ARRAY['android'::character varying, 'ios'::character varying])::text[])))
);


--
-- Name: TABLE push_devices; Type: COMMENT; Schema: auth; Owner: -
--

COMMENT ON TABLE auth.push_devices IS 'One FCM registration token per account and device; refreshed by POST /mobile/devices, deleted on logout or when FCM answers UNREGISTERED (TRACK-012 D3)';


--
-- Name: user_profile; Type: TABLE; Schema: auth; Owner: -
--

CREATE TABLE auth.user_profile (
    user_id uuid NOT NULL,
    profile_picture_url text,
    bio text,
    website_url character varying,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: user_sessions; Type: TABLE; Schema: auth; Owner: -
--

CREATE TABLE auth.user_sessions (
    session_id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    user_id uuid NOT NULL,
    provider_name character varying(50) NOT NULL,
    auth_provider_id character varying(255) NOT NULL,
    login_time timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    logout_time timestamp with time zone,
    ip_address character varying(255),
    device_id uuid,
    device_info character varying(255),
    browser character varying(255),
    device_os character varying(255),
    user_agent text,
    multifactor_enabled boolean,
    multifactor_verified boolean,
    is_active boolean DEFAULT true,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    client_type character varying(10) DEFAULT 'web'::character varying NOT NULL,
    CONSTRAINT chk_user_sessions_client_type CHECK (((client_type)::text = ANY ((ARRAY['web'::character varying, 'mobile'::character varying])::text[])))
);


--
-- Name: COLUMN user_sessions.client_type; Type: COMMENT; Schema: auth; Owner: -
--

COMMENT ON COLUMN auth.user_sessions.client_type IS 'Which client opened the session — web or mobile (TRACK-015 D2); declared by X-Client-Type, never carried in the JWT';


--
-- Name: users; Type: TABLE; Schema: auth; Owner: -
--

CREATE TABLE auth.users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    first_name character varying(100),
    last_name character varying(100),
    phone character varying(15),
    birthday date,
    email character varying(255) DEFAULT NULL::character varying,
    password character varying(255) DEFAULT NULL::character varying,
    is_active boolean DEFAULT true,
    email_verified boolean DEFAULT true,
    expiration_date timestamp with time zone,
    system_role character varying(20) DEFAULT 'user'::character varying,
    subscription_plan character varying(20) DEFAULT 'free'::character varying,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    deleted_at timestamp with time zone,
    deletion_reason text,
    locale character varying(2) DEFAULT 'es'::character varying NOT NULL,
    CONSTRAINT chk_users_locale CHECK (((locale)::text = ANY ((ARRAY['es'::character varying, 'en'::character varying])::text[]))),
    CONSTRAINT users_subscription_plan_check CHECK (((subscription_plan)::text = ANY ((ARRAY['free'::character varying, 'pro'::character varying, 'enterprise'::character varying])::text[]))),
    CONSTRAINT users_system_role_check CHECK (((system_role)::text = ANY ((ARRAY['super_admin'::character varying, 'admin'::character varying, 'user'::character varying])::text[])))
);


--
-- Name: COLUMN users.system_role; Type: COMMENT; Schema: auth; Owner: -
--

COMMENT ON COLUMN auth.users.system_role IS 'System-level role: super_admin (full access), admin (user management), user (default)';


--
-- Name: COLUMN users.subscription_plan; Type: COMMENT; Schema: auth; Owner: -
--

COMMENT ON COLUMN auth.users.subscription_plan IS 'Subscription plan affecting tenant limits: free (1 tenant), pro (5 tenants), enterprise (unlimited)';


--
-- Name: COLUMN users.deleted_at; Type: COMMENT; Schema: auth; Owner: -
--

COMMENT ON COLUMN auth.users.deleted_at IS 'Soft delete timestamp. NULL = active user, NOT NULL = deleted user';


--
-- Name: COLUMN users.deletion_reason; Type: COMMENT; Schema: auth; Owner: -
--

COMMENT ON COLUMN auth.users.deletion_reason IS 'Optional reason for user deletion';


--
-- Name: COLUMN users.locale; Type: COMMENT; Schema: auth; Owner: -
--

COMMENT ON COLUMN auth.users.locale IS 'The language this person reads the app and its emails in; set when they switch it (APP-009 D2)';


--
-- Name: email_verification_tokens email_verification_tokens_pkey; Type: CONSTRAINT; Schema: auth; Owner: -
--

ALTER TABLE ONLY auth.email_verification_tokens
    ADD CONSTRAINT email_verification_tokens_pkey PRIMARY KEY (id);


--
-- Name: email_verification_tokens email_verification_tokens_token_key; Type: CONSTRAINT; Schema: auth; Owner: -
--

ALTER TABLE ONLY auth.email_verification_tokens
    ADD CONSTRAINT email_verification_tokens_token_key UNIQUE (token);


--
-- Name: otp_requests otp_requests_pkey; Type: CONSTRAINT; Schema: auth; Owner: -
--

ALTER TABLE ONLY auth.otp_requests
    ADD CONSTRAINT otp_requests_pkey PRIMARY KEY (id);


--
-- Name: password_reset_tokens password_reset_tokens_pkey; Type: CONSTRAINT; Schema: auth; Owner: -
--

ALTER TABLE ONLY auth.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_pkey PRIMARY KEY (id);


--
-- Name: push_devices push_devices_pkey; Type: CONSTRAINT; Schema: auth; Owner: -
--

ALTER TABLE ONLY auth.push_devices
    ADD CONSTRAINT push_devices_pkey PRIMARY KEY (user_id, device_id);


--
-- Name: user_profile user_profile_pkey; Type: CONSTRAINT; Schema: auth; Owner: -
--

ALTER TABLE ONLY auth.user_profile
    ADD CONSTRAINT user_profile_pkey PRIMARY KEY (user_id);


--
-- Name: user_sessions user_sessions_pkey; Type: CONSTRAINT; Schema: auth; Owner: -
--

ALTER TABLE ONLY auth.user_sessions
    ADD CONSTRAINT user_sessions_pkey PRIMARY KEY (session_id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: auth; Owner: -
--

ALTER TABLE ONLY auth.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: idx_email_verify_expires_at; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_email_verify_expires_at ON auth.email_verification_tokens USING btree (expires_at);


--
-- Name: idx_email_verify_new_email; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_email_verify_new_email ON auth.email_verification_tokens USING btree (new_email);


--
-- Name: idx_email_verify_token; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_email_verify_token ON auth.email_verification_tokens USING btree (token);


--
-- Name: idx_email_verify_user_id; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_email_verify_user_id ON auth.email_verification_tokens USING btree (user_id);


--
-- Name: idx_forgot_expires_at; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_forgot_expires_at ON auth.password_reset_tokens USING btree (expires_at);


--
-- Name: idx_forgot_token; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_forgot_token ON auth.password_reset_tokens USING btree (token);


--
-- Name: idx_forgot_user_id; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_forgot_user_id ON auth.password_reset_tokens USING btree (user_id);


--
-- Name: idx_otp_created_at; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_otp_created_at ON auth.otp_requests USING btree (created_at);


--
-- Name: idx_otp_destination_channel; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_otp_destination_channel ON auth.otp_requests USING btree (destination, otp_channel) WHERE ((is_verified = false) AND (invalidated_at IS NULL));


--
-- Name: idx_otp_expires_at; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_otp_expires_at ON auth.otp_requests USING btree (expires_at) WHERE ((is_verified = false) AND (invalidated_at IS NULL));


--
-- Name: idx_otp_user_id; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_otp_user_id ON auth.otp_requests USING btree (user_id) WHERE (is_verified = false);


--
-- Name: idx_push_devices_token; Type: INDEX; Schema: auth; Owner: -
--

CREATE UNIQUE INDEX idx_push_devices_token ON auth.push_devices USING btree (token);


--
-- Name: idx_user_profile_created_at; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_user_profile_created_at ON auth.user_profile USING btree (created_at);


--
-- Name: idx_user_profile_updated_at; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_user_profile_updated_at ON auth.user_profile USING btree (updated_at);


--
-- Name: idx_user_sessions_created_at; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_user_sessions_created_at ON auth.user_sessions USING btree (created_at);


--
-- Name: idx_user_sessions_updated_at; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_user_sessions_updated_at ON auth.user_sessions USING btree (updated_at);


--
-- Name: idx_user_sessions_verify; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_user_sessions_verify ON auth.user_sessions USING btree (user_id, provider_name, auth_provider_id, device_id, is_active);


--
-- Name: idx_users_birthday; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_users_birthday ON auth.users USING btree (birthday);


--
-- Name: idx_users_created_at; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_users_created_at ON auth.users USING btree (created_at);


--
-- Name: idx_users_deleted_at; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_users_deleted_at ON auth.users USING btree (deleted_at);


--
-- Name: idx_users_email; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_users_email ON auth.users USING btree (email);


--
-- Name: idx_users_email_verified; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_users_email_verified ON auth.users USING btree (email_verified);


--
-- Name: idx_users_expiration_date; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_users_expiration_date ON auth.users USING btree (expiration_date);


--
-- Name: idx_users_is_active; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_users_is_active ON auth.users USING btree (is_active);


--
-- Name: idx_users_lower_email; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_users_lower_email ON auth.users USING btree (lower((email)::text));


--
-- Name: idx_users_subscription_plan; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_users_subscription_plan ON auth.users USING btree (subscription_plan);


--
-- Name: idx_users_system_role; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_users_system_role ON auth.users USING btree (system_role);


--
-- Name: idx_users_updated_at; Type: INDEX; Schema: auth; Owner: -
--

CREATE INDEX idx_users_updated_at ON auth.users USING btree (updated_at);


--
-- Name: users trg_audit_user_update; Type: TRIGGER; Schema: auth; Owner: -
--

CREATE TRIGGER trg_audit_user_update AFTER UPDATE ON auth.users FOR EACH ROW EXECUTE FUNCTION users.trg_fn_audit_user_update();


--
-- Name: email_verification_tokens email_verification_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: auth; Owner: -
--

ALTER TABLE ONLY auth.email_verification_tokens
    ADD CONSTRAINT email_verification_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;


--
-- Name: user_sessions fk_user_session_user; Type: FK CONSTRAINT; Schema: auth; Owner: -
--

ALTER TABLE ONLY auth.user_sessions
    ADD CONSTRAINT fk_user_session_user FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;


--
-- Name: otp_requests otp_requests_user_id_fkey; Type: FK CONSTRAINT; Schema: auth; Owner: -
--

ALTER TABLE ONLY auth.otp_requests
    ADD CONSTRAINT otp_requests_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;


--
-- Name: password_reset_tokens password_reset_tokens_user_id_fkey; Type: FK CONSTRAINT; Schema: auth; Owner: -
--

ALTER TABLE ONLY auth.password_reset_tokens
    ADD CONSTRAINT password_reset_tokens_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;


--
-- Name: push_devices push_devices_user_id_fkey; Type: FK CONSTRAINT; Schema: auth; Owner: -
--

ALTER TABLE ONLY auth.push_devices
    ADD CONSTRAINT push_devices_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;


--
-- Name: user_profile user_profile_user_id_fkey; Type: FK CONSTRAINT; Schema: auth; Owner: -
--

ALTER TABLE ONLY auth.user_profile
    ADD CONSTRAINT user_profile_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id);


--
-- PostgreSQL database dump complete
--
