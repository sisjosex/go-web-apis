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
-- Name: sales; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA sales;


--
-- Name: customers; Type: TABLE; Schema: sales; Owner: -
--

CREATE TABLE sales.customers (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    email character varying(255) NOT NULL,
    phone_number character varying(20) NOT NULL,
    address character varying(500),
    city character varying(100),
    state character varying(100),
    postal_code character varying(20),
    country character varying(100),
    status character varying(50) DEFAULT 'active'::character varying,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    CONSTRAINT customers_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'inactive'::character varying, 'blocked'::character varying])::text[])))
);


--
-- Name: order_batch_assignments; Type: TABLE; Schema: sales; Owner: -
--

CREATE TABLE sales.order_batch_assignments (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    order_id uuid NOT NULL,
    order_item_id uuid NOT NULL,
    product_batch_id uuid NOT NULL,
    quantity_assigned numeric(10,2) NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    sku_id uuid NOT NULL,
    CONSTRAINT order_batch_assignments_quantity_assigned_check CHECK ((quantity_assigned > (0)::numeric))
);


--
-- Name: COLUMN order_batch_assignments.sku_id; Type: COMMENT; Schema: sales; Owner: -
--

COMMENT ON COLUMN sales.order_batch_assignments.sku_id IS 'The combination this assignment moves. Duplicated from both sides on purpose: the two composite FKs make it impossible for an assignment to name an order item of one SKU and a batch of another (INV-010 D3).';


--
-- Name: order_items; Type: TABLE; Schema: sales; Owner: -
--

CREATE TABLE sales.order_items (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    order_id uuid NOT NULL,
    product_id uuid NOT NULL,
    product_sku character varying(100),
    product_name character varying(255),
    quantity integer NOT NULL,
    unit_price numeric(12,2) NOT NULL,
    line_total numeric(12,2) GENERATED ALWAYS AS (((quantity)::numeric * unit_price)) STORED,
    created_at timestamp without time zone DEFAULT now(),
    sku_id uuid NOT NULL,
    CONSTRAINT order_items_quantity_check CHECK ((quantity > 0))
);


--
-- Name: COLUMN order_items.sku_id; Type: COMMENT; Schema: sales; Owner: -
--

COMMENT ON COLUMN sales.order_items.sku_id IS 'The sellable combination this line sold. Backfilled to the product default for every row that predates INV-010; from the first generated combination on it is the SKU the client named. product_id is carried alongside it and order_items_sku_product_fkey guarantees the two agree.';


--
-- Name: order_seq; Type: SEQUENCE; Schema: sales; Owner: -
--

CREATE SEQUENCE sales.order_seq
    START WITH 1000
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: payments; Type: TABLE; Schema: sales; Owner: -
--

CREATE TABLE sales.payments (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    order_id uuid NOT NULL,
    customer_id uuid NOT NULL,
    amount numeric(12,2) NOT NULL,
    payment_method character varying(50) NOT NULL,
    status character varying(50) DEFAULT 'pending'::character varying,
    reference_number character varying(100),
    notes text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT payments_amount_check CHECK ((amount > (0)::numeric)),
    CONSTRAINT payments_payment_method_check CHECK (((payment_method)::text = ANY ((ARRAY['cash'::character varying, 'card'::character varying, 'bank_transfer'::character varying, 'check'::character varying, 'other'::character varying])::text[]))),
    CONSTRAINT payments_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'completed'::character varying, 'failed'::character varying, 'refunded'::character varying])::text[])))
);


--
-- Name: return_seq; Type: SEQUENCE; Schema: sales; Owner: -
--

CREATE SEQUENCE sales.return_seq
    START WITH 1000
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


--
-- Name: returns; Type: TABLE; Schema: sales; Owner: -
--

CREATE TABLE sales.returns (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    order_id uuid NOT NULL,
    customer_id uuid NOT NULL,
    return_number character varying(50) NOT NULL,
    total_amount numeric(12,2) NOT NULL,
    reason character varying(255),
    status character varying(50) DEFAULT 'pending'::character varying,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    CONSTRAINT returns_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'approved'::character varying, 'rejected'::character varying, 'completed'::character varying])::text[])))
);


--
-- Name: sales_orders; Type: TABLE; Schema: sales; Owner: -
--

CREATE TABLE sales.sales_orders (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id uuid NOT NULL,
    customer_id uuid NOT NULL,
    order_number character varying(50) NOT NULL,
    status character varying(50) DEFAULT 'pending'::character varying,
    sub_total numeric(12,2) DEFAULT 0 NOT NULL,
    tax_amount numeric(12,2) DEFAULT 0 NOT NULL,
    total numeric(12,2) DEFAULT 0 NOT NULL,
    discount_amount numeric(12,2) DEFAULT 0,
    shipping_address character varying(500),
    notes text,
    created_at timestamp without time zone DEFAULT now(),
    updated_at timestamp without time zone DEFAULT now(),
    CONSTRAINT sales_orders_status_check CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'confirmed'::character varying, 'shipped'::character varying, 'delivered'::character varying, 'completed'::character varying, 'cancelled'::character varying])::text[])))
);


--
-- Name: customers customers_pkey; Type: CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.customers
    ADD CONSTRAINT customers_pkey PRIMARY KEY (id);


--
-- Name: order_batch_assignments order_batch_assignments_order_item_id_product_batch_id_key; Type: CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.order_batch_assignments
    ADD CONSTRAINT order_batch_assignments_order_item_id_product_batch_id_key UNIQUE (order_item_id, product_batch_id);


--
-- Name: order_batch_assignments order_batch_assignments_pkey; Type: CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.order_batch_assignments
    ADD CONSTRAINT order_batch_assignments_pkey PRIMARY KEY (id);


--
-- Name: order_items order_items_pkey; Type: CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.order_items
    ADD CONSTRAINT order_items_pkey PRIMARY KEY (id);


--
-- Name: payments payments_pkey; Type: CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.payments
    ADD CONSTRAINT payments_pkey PRIMARY KEY (id);


--
-- Name: returns returns_pkey; Type: CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.returns
    ADD CONSTRAINT returns_pkey PRIMARY KEY (id);


--
-- Name: returns returns_return_number_key; Type: CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.returns
    ADD CONSTRAINT returns_return_number_key UNIQUE (return_number);


--
-- Name: sales_orders sales_orders_pkey; Type: CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.sales_orders
    ADD CONSTRAINT sales_orders_pkey PRIMARY KEY (id);


--
-- Name: idx_customers_email_unique; Type: INDEX; Schema: sales; Owner: -
--

CREATE UNIQUE INDEX idx_customers_email_unique ON sales.customers USING btree (tenant_id, email);


--
-- Name: idx_customers_tenant_id; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_customers_tenant_id ON sales.customers USING btree (tenant_id);


--
-- Name: idx_order_batch_assignments_batch_id; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_order_batch_assignments_batch_id ON sales.order_batch_assignments USING btree (product_batch_id);


--
-- Name: idx_order_batch_assignments_order_id; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_order_batch_assignments_order_id ON sales.order_batch_assignments USING btree (order_id);


--
-- Name: idx_order_batch_assignments_sku_id; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_order_batch_assignments_sku_id ON sales.order_batch_assignments USING btree (sku_id);


--
-- Name: idx_order_items_id_sku_id; Type: INDEX; Schema: sales; Owner: -
--

CREATE UNIQUE INDEX idx_order_items_id_sku_id ON sales.order_items USING btree (id, sku_id);


--
-- Name: idx_order_items_order_id; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_order_items_order_id ON sales.order_items USING btree (order_id);


--
-- Name: idx_order_items_product_id; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_order_items_product_id ON sales.order_items USING btree (product_id);


--
-- Name: idx_order_items_sku_id; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_order_items_sku_id ON sales.order_items USING btree (sku_id);


--
-- Name: idx_payments_customer_id; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_payments_customer_id ON sales.payments USING btree (customer_id);


--
-- Name: idx_payments_order_id; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_payments_order_id ON sales.payments USING btree (order_id);


--
-- Name: idx_payments_status; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_payments_status ON sales.payments USING btree (status);


--
-- Name: idx_returns_customer_id; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_returns_customer_id ON sales.returns USING btree (customer_id);


--
-- Name: idx_returns_order_id; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_returns_order_id ON sales.returns USING btree (order_id);


--
-- Name: idx_returns_status; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_returns_status ON sales.returns USING btree (status);


--
-- Name: idx_sales_orders_created_at; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_sales_orders_created_at ON sales.sales_orders USING btree (created_at);


--
-- Name: idx_sales_orders_customer_id; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_sales_orders_customer_id ON sales.sales_orders USING btree (customer_id);


--
-- Name: idx_sales_orders_order_number; Type: INDEX; Schema: sales; Owner: -
--

CREATE UNIQUE INDEX idx_sales_orders_order_number ON sales.sales_orders USING btree (tenant_id, order_number);


--
-- Name: idx_sales_orders_status; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_sales_orders_status ON sales.sales_orders USING btree (status);


--
-- Name: idx_sales_orders_tenant_id; Type: INDEX; Schema: sales; Owner: -
--

CREATE INDEX idx_sales_orders_tenant_id ON sales.sales_orders USING btree (tenant_id);


--
-- Name: customers customers_update_timestamp; Type: TRIGGER; Schema: sales; Owner: -
--

CREATE TRIGGER customers_update_timestamp BEFORE UPDATE ON sales.customers FOR EACH ROW EXECUTE FUNCTION sales.customers_update_timestamp();


--
-- Name: sales_orders sales_orders_update_timestamp; Type: TRIGGER; Schema: sales; Owner: -
--

CREATE TRIGGER sales_orders_update_timestamp BEFORE UPDATE ON sales.sales_orders FOR EACH ROW EXECUTE FUNCTION sales.sales_orders_update_timestamp();


--
-- Name: order_batch_assignments order_batch_assignments_batch_sku_fkey; Type: FK CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.order_batch_assignments
    ADD CONSTRAINT order_batch_assignments_batch_sku_fkey FOREIGN KEY (product_batch_id, sku_id) REFERENCES inventory.product_batches(id, sku_id);


--
-- Name: order_batch_assignments order_batch_assignments_item_sku_fkey; Type: FK CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.order_batch_assignments
    ADD CONSTRAINT order_batch_assignments_item_sku_fkey FOREIGN KEY (order_item_id, sku_id) REFERENCES sales.order_items(id, sku_id) ON DELETE CASCADE;


--
-- Name: order_batch_assignments order_batch_assignments_order_id_fkey; Type: FK CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.order_batch_assignments
    ADD CONSTRAINT order_batch_assignments_order_id_fkey FOREIGN KEY (order_id) REFERENCES sales.sales_orders(id) ON DELETE CASCADE;


--
-- Name: order_batch_assignments order_batch_assignments_order_item_id_fkey; Type: FK CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.order_batch_assignments
    ADD CONSTRAINT order_batch_assignments_order_item_id_fkey FOREIGN KEY (order_item_id) REFERENCES sales.order_items(id) ON DELETE CASCADE;


--
-- Name: order_batch_assignments order_batch_assignments_product_batch_id_fkey; Type: FK CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.order_batch_assignments
    ADD CONSTRAINT order_batch_assignments_product_batch_id_fkey FOREIGN KEY (product_batch_id) REFERENCES inventory.product_batches(id);


--
-- Name: order_items order_items_order_id_fkey; Type: FK CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.order_items
    ADD CONSTRAINT order_items_order_id_fkey FOREIGN KEY (order_id) REFERENCES sales.sales_orders(id) ON DELETE CASCADE;


--
-- Name: order_items order_items_sku_product_fkey; Type: FK CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.order_items
    ADD CONSTRAINT order_items_sku_product_fkey FOREIGN KEY (sku_id, product_id) REFERENCES inventory.product_skus(id, product_id);


--
-- Name: payments payments_customer_id_fkey; Type: FK CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.payments
    ADD CONSTRAINT payments_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES sales.customers(id);


--
-- Name: payments payments_order_id_fkey; Type: FK CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.payments
    ADD CONSTRAINT payments_order_id_fkey FOREIGN KEY (order_id) REFERENCES sales.sales_orders(id) ON DELETE CASCADE;


--
-- Name: returns returns_customer_id_fkey; Type: FK CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.returns
    ADD CONSTRAINT returns_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES sales.customers(id);


--
-- Name: returns returns_order_id_fkey; Type: FK CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.returns
    ADD CONSTRAINT returns_order_id_fkey FOREIGN KEY (order_id) REFERENCES sales.sales_orders(id) ON DELETE CASCADE;


--
-- Name: sales_orders sales_orders_customer_id_fkey; Type: FK CONSTRAINT; Schema: sales; Owner: -
--

ALTER TABLE ONLY sales.sales_orders
    ADD CONSTRAINT sales_orders_customer_id_fkey FOREIGN KEY (customer_id) REFERENCES sales.customers(id) ON DELETE RESTRICT;


--
-- PostgreSQL database dump complete
--
