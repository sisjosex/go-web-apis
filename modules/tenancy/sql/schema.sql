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
-- Name: tenancy; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA tenancy;


--
-- Name: app_invites; Type: TABLE; Schema: tenancy; Owner: -
--

CREATE TABLE tenancy.app_invites (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    level character varying(20) NOT NULL,
    channel character varying(20) NOT NULL,
    destination character varying(255),
    created_by uuid,
    sent_at timestamp with time zone,
    accepted_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT chk_app_invites_channel CHECK (((channel)::text = ANY ((ARRAY['whatsapp'::character varying, 'email'::character varying])::text[]))),
    CONSTRAINT chk_app_invites_level CHECK (((level)::text = ANY ((ARRAY['portal'::character varying, 'driver'::character varying, 'organization'::character varying])::text[])))
);


--
-- Name: TABLE app_invites; Type: COMMENT; Schema: tenancy; Owner: -
--

COMMENT ON TABLE tenancy.app_invites IS 'An app access invitation per account and business: its channel and destination, when it went out and when the person first signed in after it (TRACK-062 D3); the status is derived, never stored';


--
-- Name: gps_devices; Type: TABLE; Schema: tenancy; Owner: -
--

CREATE TABLE tenancy.gps_devices (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    token_hash bytea NOT NULL,
    tenant_id uuid NOT NULL,
    vehicle_id uuid NOT NULL,
    last_seen_at timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE gps_devices; Type: COMMENT; Schema: tenancy; Owner: -
--

COMMENT ON TABLE tenancy.gps_devices IS 'One GPS device credential per vehicle: the sha256 of its token, whose tenant and vehicle it posts for (TRACK-010)';


--
-- Name: role_permissions; Type: TABLE; Schema: tenancy; Owner: -
--

CREATE TABLE tenancy.role_permissions (
    role_id uuid NOT NULL,
    permission_code character varying(200) NOT NULL
);


--
-- Name: roles; Type: TABLE; Schema: tenancy; Owner: -
--

CREATE TABLE tenancy.roles (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    name character varying(100) NOT NULL,
    description text,
    is_system boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: tenant_modules; Type: TABLE; Schema: tenancy; Owner: -
--

CREATE TABLE tenancy.tenant_modules (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id uuid NOT NULL,
    module_code character varying(100) NOT NULL,
    is_enabled boolean DEFAULT true NOT NULL,
    config jsonb DEFAULT '{}'::jsonb NOT NULL,
    enabled_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    enabled_by uuid,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: tenant_users; Type: TABLE; Schema: tenancy; Owner: -
--

CREATE TABLE tenancy.tenant_users (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id uuid NOT NULL,
    user_id uuid NOT NULL,
    role character varying(50) DEFAULT 'member'::character varying NOT NULL,
    is_active boolean DEFAULT true,
    joined_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: tenants; Type: TABLE; Schema: tenancy; Owner: -
--

CREATE TABLE tenancy.tenants (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    slug character varying(100) NOT NULL,
    name character varying(255) NOT NULL,
    database_url text,
    schema_name character varying(63) DEFAULT 'public'::character varying,
    is_active boolean DEFAULT true,
    is_suspended boolean DEFAULT false,
    suspended_reason text,
    settings jsonb DEFAULT '{}'::jsonb,
    created_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp with time zone DEFAULT CURRENT_TIMESTAMP,
    verticals character varying(32)[] DEFAULT '{}'::character varying[] NOT NULL,
    primary_module character varying(32)
);


--
-- Name: usage_daily; Type: TABLE; Schema: tenancy; Owner: -
--

CREATE TABLE tenancy.usage_daily (
    tenant_id uuid NOT NULL,
    day date NOT NULL,
    vehicles_active integer DEFAULT 0 NOT NULL,
    trips_completed integer DEFAULT 0 NOT NULL,
    positions bigint DEFAULT 0 NOT NULL,
    riders_boarded integer DEFAULT 0 NOT NULL,
    rolled_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE usage_daily; Type: COMMENT; Schema: tenancy; Owner: -
--

COMMENT ON TABLE tenancy.usage_daily IS 'Per tenant and UTC day: vehicles that ran, trips completed, GPS points and riders boarded (TRACK-013 D3)';


--
-- Name: user_roles; Type: TABLE; Schema: tenancy; Owner: -
--

CREATE TABLE tenancy.user_roles (
    user_id uuid NOT NULL,
    role_id uuid NOT NULL,
    tenant_id uuid NOT NULL,
    assigned_by uuid NOT NULL,
    assigned_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: app_invites app_invites_pkey; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.app_invites
    ADD CONSTRAINT app_invites_pkey PRIMARY KEY (id);


--
-- Name: gps_devices gps_devices_pkey; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.gps_devices
    ADD CONSTRAINT gps_devices_pkey PRIMARY KEY (id);


--
-- Name: gps_devices gps_devices_token_hash_key; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.gps_devices
    ADD CONSTRAINT gps_devices_token_hash_key UNIQUE (token_hash);


--
-- Name: role_permissions role_permissions_pkey; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.role_permissions
    ADD CONSTRAINT role_permissions_pkey PRIMARY KEY (role_id, permission_code);


--
-- Name: roles roles_pkey; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.roles
    ADD CONSTRAINT roles_pkey PRIMARY KEY (id);


--
-- Name: roles roles_tenant_id_name_key; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.roles
    ADD CONSTRAINT roles_tenant_id_name_key UNIQUE (tenant_id, name);


--
-- Name: tenant_modules tenant_modules_pkey; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.tenant_modules
    ADD CONSTRAINT tenant_modules_pkey PRIMARY KEY (id);


--
-- Name: tenant_users tenant_users_pkey; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.tenant_users
    ADD CONSTRAINT tenant_users_pkey PRIMARY KEY (id);


--
-- Name: tenant_users tenant_users_tenant_id_user_id_key; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.tenant_users
    ADD CONSTRAINT tenant_users_tenant_id_user_id_key UNIQUE (tenant_id, user_id);


--
-- Name: tenants tenants_pkey; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.tenants
    ADD CONSTRAINT tenants_pkey PRIMARY KEY (id);


--
-- Name: tenants tenants_slug_key; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.tenants
    ADD CONSTRAINT tenants_slug_key UNIQUE (slug);


--
-- Name: gps_devices uk_gps_devices_vehicle; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.gps_devices
    ADD CONSTRAINT uk_gps_devices_vehicle UNIQUE (tenant_id, vehicle_id);


--
-- Name: app_invites uq_app_invites_tenant_user; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.app_invites
    ADD CONSTRAINT uq_app_invites_tenant_user UNIQUE (tenant_id, user_id);


--
-- Name: tenant_modules uq_tenant_module; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.tenant_modules
    ADD CONSTRAINT uq_tenant_module UNIQUE (tenant_id, module_code);


--
-- Name: usage_daily usage_daily_pkey; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.usage_daily
    ADD CONSTRAINT usage_daily_pkey PRIMARY KEY (tenant_id, day);


--
-- Name: user_roles user_roles_pkey; Type: CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.user_roles
    ADD CONSTRAINT user_roles_pkey PRIMARY KEY (user_id, role_id);


--
-- Name: idx_app_invites_user_open; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_app_invites_user_open ON tenancy.app_invites USING btree (user_id) WHERE (accepted_at IS NULL);


--
-- Name: idx_role_permissions_role_id; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_role_permissions_role_id ON tenancy.role_permissions USING btree (role_id);


--
-- Name: idx_roles_tenant_id; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_roles_tenant_id ON tenancy.roles USING btree (tenant_id);


--
-- Name: idx_tenant_modules_is_enabled; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_tenant_modules_is_enabled ON tenancy.tenant_modules USING btree (is_enabled);


--
-- Name: idx_tenant_modules_module_code; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_tenant_modules_module_code ON tenancy.tenant_modules USING btree (module_code);


--
-- Name: idx_tenant_modules_tenant_id; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_tenant_modules_tenant_id ON tenancy.tenant_modules USING btree (tenant_id);


--
-- Name: idx_tenant_users_is_active; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_tenant_users_is_active ON tenancy.tenant_users USING btree (is_active);


--
-- Name: idx_tenant_users_joined_at; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_tenant_users_joined_at ON tenancy.tenant_users USING btree (joined_at);


--
-- Name: idx_tenant_users_role; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_tenant_users_role ON tenancy.tenant_users USING btree (role);


--
-- Name: idx_tenant_users_tenant_id; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_tenant_users_tenant_id ON tenancy.tenant_users USING btree (tenant_id);


--
-- Name: idx_tenant_users_user_id; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_tenant_users_user_id ON tenancy.tenant_users USING btree (user_id);


--
-- Name: idx_tenants_created_at; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_tenants_created_at ON tenancy.tenants USING btree (created_at);


--
-- Name: idx_tenants_is_active; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_tenants_is_active ON tenancy.tenants USING btree (is_active);


--
-- Name: idx_tenants_is_suspended; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_tenants_is_suspended ON tenancy.tenants USING btree (is_suspended);


--
-- Name: idx_tenants_slug; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_tenants_slug ON tenancy.tenants USING btree (slug);


--
-- Name: idx_usage_daily_day; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_usage_daily_day ON tenancy.usage_daily USING btree (day);


--
-- Name: idx_user_roles_role_id; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_user_roles_role_id ON tenancy.user_roles USING btree (role_id);


--
-- Name: idx_user_roles_user_tenant; Type: INDEX; Schema: tenancy; Owner: -
--

CREATE INDEX idx_user_roles_user_tenant ON tenancy.user_roles USING btree (user_id, tenant_id);


--
-- Name: tenant_modules trigger_tenant_modules_updated_at; Type: TRIGGER; Schema: tenancy; Owner: -
--

CREATE TRIGGER trigger_tenant_modules_updated_at BEFORE UPDATE ON tenancy.tenant_modules FOR EACH ROW EXECUTE FUNCTION tenancy.update_tenant_module_updated_at();


--
-- Name: tenants trigger_update_tenant_updated_at; Type: TRIGGER; Schema: tenancy; Owner: -
--

CREATE TRIGGER trigger_update_tenant_updated_at BEFORE UPDATE ON tenancy.tenants FOR EACH ROW EXECUTE FUNCTION tenancy.update_tenant_updated_at();


--
-- Name: app_invites app_invites_created_by_fkey; Type: FK CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.app_invites
    ADD CONSTRAINT app_invites_created_by_fkey FOREIGN KEY (created_by) REFERENCES auth.users(id) ON DELETE SET NULL;


--
-- Name: app_invites app_invites_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.app_invites
    ADD CONSTRAINT app_invites_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenancy.tenants(id) ON DELETE CASCADE;


--
-- Name: app_invites app_invites_user_id_fkey; Type: FK CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.app_invites
    ADD CONSTRAINT app_invites_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;


--
-- Name: gps_devices gps_devices_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.gps_devices
    ADD CONSTRAINT gps_devices_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenancy.tenants(id) ON DELETE CASCADE;


--
-- Name: role_permissions role_permissions_role_id_fkey; Type: FK CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.role_permissions
    ADD CONSTRAINT role_permissions_role_id_fkey FOREIGN KEY (role_id) REFERENCES tenancy.roles(id) ON DELETE CASCADE;


--
-- Name: roles roles_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.roles
    ADD CONSTRAINT roles_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenancy.tenants(id) ON DELETE CASCADE;


--
-- Name: tenant_modules tenant_modules_enabled_by_fkey; Type: FK CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.tenant_modules
    ADD CONSTRAINT tenant_modules_enabled_by_fkey FOREIGN KEY (enabled_by) REFERENCES auth.users(id) ON DELETE SET NULL;


--
-- Name: tenant_modules tenant_modules_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.tenant_modules
    ADD CONSTRAINT tenant_modules_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenancy.tenants(id) ON DELETE CASCADE;


--
-- Name: tenant_users tenant_users_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.tenant_users
    ADD CONSTRAINT tenant_users_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenancy.tenants(id) ON DELETE CASCADE;


--
-- Name: tenant_users tenant_users_user_id_fkey; Type: FK CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.tenant_users
    ADD CONSTRAINT tenant_users_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;


--
-- Name: usage_daily usage_daily_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.usage_daily
    ADD CONSTRAINT usage_daily_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenancy.tenants(id) ON DELETE CASCADE;


--
-- Name: user_roles user_roles_role_id_fkey; Type: FK CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.user_roles
    ADD CONSTRAINT user_roles_role_id_fkey FOREIGN KEY (role_id) REFERENCES tenancy.roles(id) ON DELETE CASCADE;


--
-- Name: user_roles user_roles_tenant_id_fkey; Type: FK CONSTRAINT; Schema: tenancy; Owner: -
--

ALTER TABLE ONLY tenancy.user_roles
    ADD CONSTRAINT user_roles_tenant_id_fkey FOREIGN KEY (tenant_id) REFERENCES tenancy.tenants(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--
