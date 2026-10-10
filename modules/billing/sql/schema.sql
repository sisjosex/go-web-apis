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
-- Name: billing; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA billing;


--
-- Name: payments; Type: TABLE; Schema: billing; Owner: -
--

CREATE TABLE billing.payments (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    subscription_id uuid NOT NULL,
    user_id uuid NOT NULL,
    amount numeric(10,2) NOT NULL,
    currency character varying(3) DEFAULT 'USD'::character varying NOT NULL,
    status character varying(20) DEFAULT 'pending'::character varying NOT NULL,
    provider character varying(50),
    provider_payment_id character varying(255),
    paid_at timestamp with time zone,
    period_start timestamp with time zone,
    period_end timestamp with time zone,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    tenant_id uuid,
    plan character varying(20),
    billing_cycle character varying(10),
    reference character varying(255),
    purpose character varying(30) DEFAULT 'subscription'::character varying NOT NULL,
    confirmed_by uuid,
    reject_reason text,
    rejected_by uuid,
    CONSTRAINT payments_billing_cycle_check CHECK (((billing_cycle)::text = ANY ((ARRAY['monthly'::character varying, 'annual'::character varying])::text[]))),
    CONSTRAINT payments_plan_check CHECK (((plan)::text = ANY ((ARRAY['free'::character varying, 'pro'::character varying, 'enterprise'::character varying])::text[]))),
    CONSTRAINT payments_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'notified'::character varying, 'completed'::character varying, 'failed'::character varying, 'refunded'::character varying, 'rejected'::character varying])::text[])))
);


--
-- Name: TABLE payments; Type: COMMENT; Schema: billing; Owner: -
--

COMMENT ON TABLE billing.payments IS 'Payment history for subscriptions';


--
-- Name: COLUMN payments.purpose; Type: COMMENT; Schema: billing; Owner: -
--

COMMENT ON COLUMN billing.payments.purpose IS 'What the payment is for: subscription today; a future sale (POS, web shop) reuses the same notice and confirmation (BILLING-001 D3)';


--
-- Name: plan_limits; Type: TABLE; Schema: billing; Owner: -
--

CREATE TABLE billing.plan_limits (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    plan character varying(20) NOT NULL,
    feature character varying(100) NOT NULL,
    limit_value integer DEFAULT '-1'::integer NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT plan_limits_plan_check CHECK (((plan)::text = ANY ((ARRAY['free'::character varying, 'pro'::character varying, 'enterprise'::character varying])::text[])))
);


--
-- Name: TABLE plan_limits; Type: COMMENT; Schema: billing; Owner: -
--

COMMENT ON TABLE billing.plan_limits IS 'Plan feature limit definitions — seeded, checked by SPs and Go services';


--
-- Name: plan_prices; Type: TABLE; Schema: billing; Owner: -
--

CREATE TABLE billing.plan_prices (
    plan character varying(20) NOT NULL,
    billing_cycle character varying(10) NOT NULL,
    amount numeric(10,2) NOT NULL,
    currency character varying(3) DEFAULT 'BOB'::character varying NOT NULL,
    qr_image_url text,
    CONSTRAINT plan_prices_amount_check CHECK ((amount > (0)::numeric)),
    CONSTRAINT plan_prices_billing_cycle_check CHECK (((billing_cycle)::text = ANY ((ARRAY['monthly'::character varying, 'annual'::character varying])::text[]))),
    CONSTRAINT plan_prices_plan_check CHECK (((plan)::text = ANY ((ARRAY['pro'::character varying, 'enterprise'::character varying])::text[])))
);


--
-- Name: TABLE plan_prices; Type: COMMENT; Schema: billing; Owner: -
--

COMMENT ON TABLE billing.plan_prices IS 'What a plan costs per cycle; annual = 10 months (two free). Enterprise has no row: it is sold by contact (BILLING-001 D4)';


--
-- Name: COLUMN plan_prices.qr_image_url; Type: COMMENT; Schema: billing; Owner: -
--

COMMENT ON COLUMN billing.plan_prices.qr_image_url IS 'The bank QR for this exact amount, made once in the bank''s app (BILLING-002 D1); NULL falls back to the configured QR';


--
-- Name: subscription_adjustments; Type: TABLE; Schema: billing; Owner: -
--

CREATE TABLE billing.subscription_adjustments (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    before jsonb,
    after jsonb NOT NULL,
    reason text NOT NULL,
    adjusted_by uuid,
    created_at timestamp with time zone DEFAULT now() NOT NULL
);


--
-- Name: TABLE subscription_adjustments; Type: COMMENT; Schema: billing; Owner: -
--

COMMENT ON TABLE billing.subscription_adjustments IS 'Every hand change of a business''s plan by the platform: before and after, the reason and who (BILLING-002 D3)';


--
-- Name: subscriptions; Type: TABLE; Schema: billing; Owner: -
--

CREATE TABLE billing.subscriptions (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    user_id uuid,
    plan character varying(20) DEFAULT 'free'::character varying NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    started_at timestamp with time zone DEFAULT now() NOT NULL,
    expires_at timestamp with time zone,
    canceled_at timestamp with time zone,
    stripe_sub_id character varying(255),
    stripe_customer_id character varying(255),
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    updated_at timestamp with time zone DEFAULT now() NOT NULL,
    tenant_id uuid,
    billing_cycle character varying(10) DEFAULT 'monthly'::character varying NOT NULL,
    CONSTRAINT chk_subscription_owner CHECK (((user_id IS NOT NULL) OR (tenant_id IS NOT NULL))),
    CONSTRAINT subscriptions_billing_cycle_check CHECK (((billing_cycle)::text = ANY ((ARRAY['monthly'::character varying, 'annual'::character varying])::text[]))),
    CONSTRAINT subscriptions_plan_check CHECK (((plan)::text = ANY ((ARRAY['free'::character varying, 'pro'::character varying, 'enterprise'::character varying])::text[]))),
    CONSTRAINT subscriptions_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'expired'::character varying, 'canceled'::character varying, 'trial'::character varying])::text[])))
);


--
-- Name: TABLE subscriptions; Type: COMMENT; Schema: billing; Owner: -
--

COMMENT ON TABLE billing.subscriptions IS 'User subscription records — one active per user';


--
-- Name: COLUMN subscriptions.tenant_id; Type: COMMENT; Schema: billing; Owner: -
--

COMMENT ON COLUMN billing.subscriptions.tenant_id IS 'The business the plan is for (BILLING-001 D1); NULL on a user''s own row, which only limits how many businesses they create';


--
-- Name: payments payments_pkey; Type: CONSTRAINT; Schema: billing; Owner: -
--

ALTER TABLE ONLY billing.payments
    ADD CONSTRAINT payments_pkey PRIMARY KEY (id);


--
-- Name: plan_limits plan_limits_pkey; Type: CONSTRAINT; Schema: billing; Owner: -
--

ALTER TABLE ONLY billing.plan_limits
    ADD CONSTRAINT plan_limits_pkey PRIMARY KEY (id);


--
-- Name: plan_limits plan_limits_plan_feature_key; Type: CONSTRAINT; Schema: billing; Owner: -
--

ALTER TABLE ONLY billing.plan_limits
    ADD CONSTRAINT plan_limits_plan_feature_key UNIQUE (plan, feature);


--
-- Name: plan_prices plan_prices_pkey; Type: CONSTRAINT; Schema: billing; Owner: -
--

ALTER TABLE ONLY billing.plan_prices
    ADD CONSTRAINT plan_prices_pkey PRIMARY KEY (plan, billing_cycle);


--
-- Name: subscription_adjustments subscription_adjustments_pkey; Type: CONSTRAINT; Schema: billing; Owner: -
--

ALTER TABLE ONLY billing.subscription_adjustments
    ADD CONSTRAINT subscription_adjustments_pkey PRIMARY KEY (id);


--
-- Name: subscriptions subscriptions_pkey; Type: CONSTRAINT; Schema: billing; Owner: -
--

ALTER TABLE ONLY billing.subscriptions
    ADD CONSTRAINT subscriptions_pkey PRIMARY KEY (id);


--
-- Name: idx_billing_payments_notified; Type: INDEX; Schema: billing; Owner: -
--

CREATE INDEX idx_billing_payments_notified ON billing.payments USING btree (created_at) WHERE ((status)::text = 'notified'::text);


--
-- Name: idx_billing_payments_status; Type: INDEX; Schema: billing; Owner: -
--

CREATE INDEX idx_billing_payments_status ON billing.payments USING btree (status);


--
-- Name: idx_billing_payments_subscription_id; Type: INDEX; Schema: billing; Owner: -
--

CREATE INDEX idx_billing_payments_subscription_id ON billing.payments USING btree (subscription_id);


--
-- Name: idx_billing_payments_user_id; Type: INDEX; Schema: billing; Owner: -
--

CREATE INDEX idx_billing_payments_user_id ON billing.payments USING btree (user_id);


--
-- Name: idx_billing_subscription_adjustments_tenant; Type: INDEX; Schema: billing; Owner: -
--

CREATE INDEX idx_billing_subscription_adjustments_tenant ON billing.subscription_adjustments USING btree (tenant_id, created_at DESC);


--
-- Name: idx_billing_subscriptions_expiry; Type: INDEX; Schema: billing; Owner: -
--

CREATE INDEX idx_billing_subscriptions_expiry ON billing.subscriptions USING btree (expires_at) WHERE ((tenant_id IS NOT NULL) AND ((status)::text = 'active'::text) AND ((plan)::text <> 'free'::text));


--
-- Name: idx_billing_subscriptions_plan; Type: INDEX; Schema: billing; Owner: -
--

CREATE INDEX idx_billing_subscriptions_plan ON billing.subscriptions USING btree (plan);


--
-- Name: idx_billing_subscriptions_status; Type: INDEX; Schema: billing; Owner: -
--

CREATE INDEX idx_billing_subscriptions_status ON billing.subscriptions USING btree (status);


--
-- Name: idx_billing_subscriptions_user_id; Type: INDEX; Schema: billing; Owner: -
--

CREATE INDEX idx_billing_subscriptions_user_id ON billing.subscriptions USING btree (user_id);


--
-- Name: uq_billing_subscriptions_tenant_active; Type: INDEX; Schema: billing; Owner: -
--

CREATE UNIQUE INDEX uq_billing_subscriptions_tenant_active ON billing.subscriptions USING btree (tenant_id) WHERE ((tenant_id IS NOT NULL) AND ((status)::text = ANY ((ARRAY['active'::character varying, 'trial'::character varying])::text[])));


--
-- Name: uq_billing_subscriptions_user_active; Type: INDEX; Schema: billing; Owner: -
--

CREATE UNIQUE INDEX uq_billing_subscriptions_user_active ON billing.subscriptions USING btree (user_id) WHERE ((status)::text = ANY ((ARRAY['active'::character varying, 'trial'::character varying])::text[]));


--
-- Name: payments payments_confirmed_by_fkey; Type: FK CONSTRAINT; Schema: billing; Owner: -
--

ALTER TABLE ONLY billing.payments
    ADD CONSTRAINT payments_confirmed_by_fkey FOREIGN KEY (confirmed_by) REFERENCES auth.users(id) ON DELETE SET NULL;


--
-- Name: payments payments_rejected_by_fkey; Type: FK CONSTRAINT; Schema: billing; Owner: -
--

ALTER TABLE ONLY billing.payments
    ADD CONSTRAINT payments_rejected_by_fkey FOREIGN KEY (rejected_by) REFERENCES auth.users(id) ON DELETE SET NULL;


--
-- Name: payments payments_subscription_id_fkey; Type: FK CONSTRAINT; Schema: billing; Owner: -
--

ALTER TABLE ONLY billing.payments
    ADD CONSTRAINT payments_subscription_id_fkey FOREIGN KEY (subscription_id) REFERENCES billing.subscriptions(id) ON DELETE CASCADE;


--
-- Name: payments payments_user_id_fkey; Type: FK CONSTRAINT; Schema: billing; Owner: -
--

ALTER TABLE ONLY billing.payments
    ADD CONSTRAINT payments_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;


--
-- Name: subscription_adjustments subscription_adjustments_adjusted_by_fkey; Type: FK CONSTRAINT; Schema: billing; Owner: -
--

ALTER TABLE ONLY billing.subscription_adjustments
    ADD CONSTRAINT subscription_adjustments_adjusted_by_fkey FOREIGN KEY (adjusted_by) REFERENCES auth.users(id) ON DELETE SET NULL;


--
-- Name: subscriptions subscriptions_user_id_fkey; Type: FK CONSTRAINT; Schema: billing; Owner: -
--

ALTER TABLE ONLY billing.subscriptions
    ADD CONSTRAINT subscriptions_user_id_fkey FOREIGN KEY (user_id) REFERENCES auth.users(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--
