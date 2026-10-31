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
-- Name: purchasing; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA purchasing;


--
-- Name: product_batches; Type: TABLE; Schema: purchasing; Owner: -
--

CREATE TABLE purchasing.product_batches (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id uuid NOT NULL,
    product_id uuid NOT NULL,
    batch_number character varying(100) NOT NULL,
    quantity integer DEFAULT 0 NOT NULL,
    unit_cost numeric(12,2) NOT NULL,
    receipt_date timestamp without time zone NOT NULL,
    expiration_date timestamp without time zone,
    status character varying(50) DEFAULT 'received'::character varying,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: purchase_order_invoices; Type: TABLE; Schema: purchasing; Owner: -
--

CREATE TABLE purchasing.purchase_order_invoices (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    purchase_order_id uuid NOT NULL,
    invoice_number character varying(100) NOT NULL,
    invoice_date date NOT NULL,
    invoice_amount numeric(12,2) NOT NULL,
    tax_amount numeric(12,2) DEFAULT 0,
    due_date date NOT NULL,
    status character varying(20) DEFAULT 'received'::character varying,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: purchase_order_items; Type: TABLE; Schema: purchasing; Owner: -
--

CREATE TABLE purchasing.purchase_order_items (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    purchase_order_id uuid NOT NULL,
    product_id uuid NOT NULL,
    quantity numeric(14,3) NOT NULL,
    unit_cost numeric(12,2) NOT NULL,
    received_quantity numeric(14,3) DEFAULT 0,
    status character varying(20) DEFAULT 'pending'::character varying,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    line_total numeric(14,2) GENERATED ALWAYS AS ((quantity * unit_cost)) STORED,
    product_unit_id uuid,
    unit_factor numeric(14,3) DEFAULT 1 NOT NULL,
    base_quantity numeric(14,3) GENERATED ALWAYS AS ((quantity * unit_factor)) STORED,
    CONSTRAINT purchase_order_items_quantity_check CHECK ((quantity > (0)::numeric)),
    CONSTRAINT purchase_order_items_received_quantity_check CHECK ((received_quantity >= (0)::numeric))
);


--
-- Name: purchase_order_receipts; Type: TABLE; Schema: purchasing; Owner: -
--

CREATE TABLE purchasing.purchase_order_receipts (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    purchase_order_id uuid NOT NULL,
    receipt_number character varying(50) NOT NULL,
    receipt_date timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    received_by uuid,
    notes text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: purchase_orders; Type: TABLE; Schema: purchasing; Owner: -
--

CREATE TABLE purchasing.purchase_orders (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id uuid NOT NULL,
    supplier_id uuid NOT NULL,
    po_number character varying(50) NOT NULL,
    status character varying(20) DEFAULT 'draft'::character varying,
    order_date timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    expected_delivery_date date,
    total_amount numeric(12,2) DEFAULT 0,
    paid_amount numeric(12,2) DEFAULT 0,
    notes text,
    created_by uuid,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: request_for_quotes; Type: TABLE; Schema: purchasing; Owner: -
--

CREATE TABLE purchasing.request_for_quotes (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id uuid NOT NULL,
    rfq_number character varying(100) NOT NULL,
    status character varying(50) DEFAULT 'draft'::character varying,
    notes text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: rfq_items; Type: TABLE; Schema: purchasing; Owner: -
--

CREATE TABLE purchasing.rfq_items (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    rfq_id uuid NOT NULL,
    product_id uuid NOT NULL,
    quantity integer NOT NULL,
    description text,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: rfq_responses; Type: TABLE; Schema: purchasing; Owner: -
--

CREATE TABLE purchasing.rfq_responses (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    rfq_id uuid NOT NULL,
    supplier_id uuid NOT NULL,
    total_price numeric(12,2) NOT NULL,
    delivery_days integer,
    payment_terms integer DEFAULT 30,
    notes text,
    selected boolean DEFAULT false,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: supplier_products; Type: TABLE; Schema: purchasing; Owner: -
--

CREATE TABLE purchasing.supplier_products (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    supplier_id uuid NOT NULL,
    product_id uuid NOT NULL,
    supplier_sku character varying(100),
    cost_per_unit numeric(12,2) NOT NULL,
    min_order_qty integer DEFAULT 1,
    lead_time_days integer DEFAULT 3,
    is_active boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: supplier_rates; Type: TABLE; Schema: purchasing; Owner: -
--

CREATE TABLE purchasing.supplier_rates (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    supplier_id uuid NOT NULL,
    product_id uuid NOT NULL,
    unit_cost numeric(12,2) NOT NULL,
    payment_terms integer DEFAULT 30,
    last_price_date timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: suppliers; Type: TABLE; Schema: purchasing; Owner: -
--

CREATE TABLE purchasing.suppliers (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id uuid NOT NULL,
    name character varying(255) NOT NULL,
    contact_person character varying(255),
    email character varying(255),
    phone character varying(20),
    address text,
    payment_terms integer DEFAULT 30,
    is_active boolean DEFAULT true,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    tax_id character varying(30),
    legal_name character varying(255)
);


--
-- Name: product_batches product_batches_pkey; Type: CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.product_batches
    ADD CONSTRAINT product_batches_pkey PRIMARY KEY (id);


--
-- Name: purchase_order_invoices purchase_order_invoices_pkey; Type: CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.purchase_order_invoices
    ADD CONSTRAINT purchase_order_invoices_pkey PRIMARY KEY (id);


--
-- Name: purchase_order_items purchase_order_items_pkey; Type: CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.purchase_order_items
    ADD CONSTRAINT purchase_order_items_pkey PRIMARY KEY (id);


--
-- Name: purchase_order_receipts purchase_order_receipts_pkey; Type: CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.purchase_order_receipts
    ADD CONSTRAINT purchase_order_receipts_pkey PRIMARY KEY (id);


--
-- Name: purchase_orders purchase_orders_pkey; Type: CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.purchase_orders
    ADD CONSTRAINT purchase_orders_pkey PRIMARY KEY (id);


--
-- Name: request_for_quotes request_for_quotes_pkey; Type: CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.request_for_quotes
    ADD CONSTRAINT request_for_quotes_pkey PRIMARY KEY (id);


--
-- Name: rfq_items rfq_items_pkey; Type: CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.rfq_items
    ADD CONSTRAINT rfq_items_pkey PRIMARY KEY (id);


--
-- Name: rfq_responses rfq_responses_pkey; Type: CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.rfq_responses
    ADD CONSTRAINT rfq_responses_pkey PRIMARY KEY (id);


--
-- Name: supplier_products supplier_products_pkey; Type: CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.supplier_products
    ADD CONSTRAINT supplier_products_pkey PRIMARY KEY (id);


--
-- Name: supplier_rates supplier_rates_pkey; Type: CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.supplier_rates
    ADD CONSTRAINT supplier_rates_pkey PRIMARY KEY (id);


--
-- Name: suppliers suppliers_pkey; Type: CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.suppliers
    ADD CONSTRAINT suppliers_pkey PRIMARY KEY (id);


--
-- Name: idx_po_invoice_number; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE UNIQUE INDEX idx_po_invoice_number ON purchasing.purchase_order_invoices USING btree (purchase_order_id, invoice_number);


--
-- Name: idx_po_invoices_po; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_po_invoices_po ON purchasing.purchase_order_invoices USING btree (purchase_order_id);


--
-- Name: idx_po_invoices_status; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_po_invoices_status ON purchasing.purchase_order_invoices USING btree (status);


--
-- Name: idx_po_items_po; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_po_items_po ON purchasing.purchase_order_items USING btree (purchase_order_id);


--
-- Name: idx_po_items_product; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_po_items_product ON purchasing.purchase_order_items USING btree (product_id);


--
-- Name: idx_po_items_status; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_po_items_status ON purchasing.purchase_order_items USING btree (status);


--
-- Name: idx_po_receipt_number; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE UNIQUE INDEX idx_po_receipt_number ON purchasing.purchase_order_receipts USING btree (purchase_order_id, receipt_number);


--
-- Name: idx_po_receipts_po; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_po_receipts_po ON purchasing.purchase_order_receipts USING btree (purchase_order_id);


--
-- Name: idx_product_batches_batch_unique; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE UNIQUE INDEX idx_product_batches_batch_unique ON purchasing.product_batches USING btree (tenant_id, product_id, batch_number);


--
-- Name: idx_product_batches_expiration; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_product_batches_expiration ON purchasing.product_batches USING btree (expiration_date);


--
-- Name: idx_product_batches_product; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_product_batches_product ON purchasing.product_batches USING btree (product_id);


--
-- Name: idx_product_batches_receipt_date; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_product_batches_receipt_date ON purchasing.product_batches USING btree (receipt_date);


--
-- Name: idx_product_batches_status; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_product_batches_status ON purchasing.product_batches USING btree (status);


--
-- Name: idx_product_batches_tenant; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_product_batches_tenant ON purchasing.product_batches USING btree (tenant_id);


--
-- Name: idx_purchase_orders_number; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE UNIQUE INDEX idx_purchase_orders_number ON purchasing.purchase_orders USING btree (tenant_id, po_number);


--
-- Name: idx_purchase_orders_status; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_purchase_orders_status ON purchasing.purchase_orders USING btree (status);


--
-- Name: idx_purchase_orders_supplier; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_purchase_orders_supplier ON purchasing.purchase_orders USING btree (supplier_id);


--
-- Name: idx_purchase_orders_tenant; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_purchase_orders_tenant ON purchasing.purchase_orders USING btree (tenant_id);


--
-- Name: idx_request_for_quotes_status; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_request_for_quotes_status ON purchasing.request_for_quotes USING btree (status);


--
-- Name: idx_request_for_quotes_tenant; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_request_for_quotes_tenant ON purchasing.request_for_quotes USING btree (tenant_id);


--
-- Name: idx_rfq_items_product; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_rfq_items_product ON purchasing.rfq_items USING btree (product_id);


--
-- Name: idx_rfq_items_rfq; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_rfq_items_rfq ON purchasing.rfq_items USING btree (rfq_id);


--
-- Name: idx_rfq_number; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE UNIQUE INDEX idx_rfq_number ON purchasing.request_for_quotes USING btree (tenant_id, rfq_number);


--
-- Name: idx_rfq_responses_price; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_rfq_responses_price ON purchasing.rfq_responses USING btree (total_price);


--
-- Name: idx_rfq_responses_rfq; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_rfq_responses_rfq ON purchasing.rfq_responses USING btree (rfq_id);


--
-- Name: idx_rfq_responses_supplier; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_rfq_responses_supplier ON purchasing.rfq_responses USING btree (supplier_id);


--
-- Name: idx_supplier_products_product; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_supplier_products_product ON purchasing.supplier_products USING btree (product_id);


--
-- Name: idx_supplier_products_supplier; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_supplier_products_supplier ON purchasing.supplier_products USING btree (supplier_id);


--
-- Name: idx_supplier_products_unique; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE UNIQUE INDEX idx_supplier_products_unique ON purchasing.supplier_products USING btree (supplier_id, product_id);


--
-- Name: idx_supplier_rates_cost; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_supplier_rates_cost ON purchasing.supplier_rates USING btree (unit_cost);


--
-- Name: idx_supplier_rates_product; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_supplier_rates_product ON purchasing.supplier_rates USING btree (product_id);


--
-- Name: idx_supplier_rates_supplier; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_supplier_rates_supplier ON purchasing.supplier_rates USING btree (supplier_id);


--
-- Name: idx_supplier_rates_unique; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE UNIQUE INDEX idx_supplier_rates_unique ON purchasing.supplier_rates USING btree (supplier_id, product_id);


--
-- Name: idx_suppliers_active; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_suppliers_active ON purchasing.suppliers USING btree (is_active);


--
-- Name: idx_suppliers_tenant; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_suppliers_tenant ON purchasing.suppliers USING btree (tenant_id);


--
-- Name: idx_suppliers_tenant_active_name; Type: INDEX; Schema: purchasing; Owner: -
--

CREATE INDEX idx_suppliers_tenant_active_name ON purchasing.suppliers USING btree (tenant_id, is_active, name);


--
-- Name: product_batches product_batches_product_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.product_batches
    ADD CONSTRAINT product_batches_product_id_fkey FOREIGN KEY (product_id) REFERENCES inventory.products(id) ON DELETE CASCADE;


--
-- Name: purchase_order_invoices purchase_order_invoices_purchase_order_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.purchase_order_invoices
    ADD CONSTRAINT purchase_order_invoices_purchase_order_id_fkey FOREIGN KEY (purchase_order_id) REFERENCES purchasing.purchase_orders(id) ON DELETE CASCADE;


--
-- Name: purchase_order_items purchase_order_items_product_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.purchase_order_items
    ADD CONSTRAINT purchase_order_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES inventory.products(id);


--
-- Name: purchase_order_items purchase_order_items_product_unit_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.purchase_order_items
    ADD CONSTRAINT purchase_order_items_product_unit_id_fkey FOREIGN KEY (product_unit_id) REFERENCES inventory.product_units(id);


--
-- Name: purchase_order_items purchase_order_items_purchase_order_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.purchase_order_items
    ADD CONSTRAINT purchase_order_items_purchase_order_id_fkey FOREIGN KEY (purchase_order_id) REFERENCES purchasing.purchase_orders(id) ON DELETE CASCADE;


--
-- Name: purchase_order_receipts purchase_order_receipts_purchase_order_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.purchase_order_receipts
    ADD CONSTRAINT purchase_order_receipts_purchase_order_id_fkey FOREIGN KEY (purchase_order_id) REFERENCES purchasing.purchase_orders(id) ON DELETE CASCADE;


--
-- Name: purchase_order_receipts purchase_order_receipts_received_by_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.purchase_order_receipts
    ADD CONSTRAINT purchase_order_receipts_received_by_fkey FOREIGN KEY (received_by) REFERENCES auth.users(id);


--
-- Name: purchase_orders purchase_orders_created_by_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.purchase_orders
    ADD CONSTRAINT purchase_orders_created_by_fkey FOREIGN KEY (created_by) REFERENCES auth.users(id);


--
-- Name: purchase_orders purchase_orders_supplier_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.purchase_orders
    ADD CONSTRAINT purchase_orders_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES purchasing.suppliers(id) ON DELETE RESTRICT;


--
-- Name: rfq_items rfq_items_product_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.rfq_items
    ADD CONSTRAINT rfq_items_product_id_fkey FOREIGN KEY (product_id) REFERENCES inventory.products(id) ON DELETE CASCADE;


--
-- Name: rfq_items rfq_items_rfq_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.rfq_items
    ADD CONSTRAINT rfq_items_rfq_id_fkey FOREIGN KEY (rfq_id) REFERENCES purchasing.request_for_quotes(id) ON DELETE CASCADE;


--
-- Name: rfq_responses rfq_responses_rfq_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.rfq_responses
    ADD CONSTRAINT rfq_responses_rfq_id_fkey FOREIGN KEY (rfq_id) REFERENCES purchasing.request_for_quotes(id) ON DELETE CASCADE;


--
-- Name: rfq_responses rfq_responses_supplier_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.rfq_responses
    ADD CONSTRAINT rfq_responses_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES purchasing.suppliers(id) ON DELETE CASCADE;


--
-- Name: supplier_products supplier_products_product_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.supplier_products
    ADD CONSTRAINT supplier_products_product_id_fkey FOREIGN KEY (product_id) REFERENCES inventory.products(id) ON DELETE CASCADE;


--
-- Name: supplier_products supplier_products_supplier_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.supplier_products
    ADD CONSTRAINT supplier_products_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES purchasing.suppliers(id) ON DELETE CASCADE;


--
-- Name: supplier_rates supplier_rates_product_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.supplier_rates
    ADD CONSTRAINT supplier_rates_product_id_fkey FOREIGN KEY (product_id) REFERENCES inventory.products(id) ON DELETE CASCADE;


--
-- Name: supplier_rates supplier_rates_supplier_id_fkey; Type: FK CONSTRAINT; Schema: purchasing; Owner: -
--

ALTER TABLE ONLY purchasing.supplier_rates
    ADD CONSTRAINT supplier_rates_supplier_id_fkey FOREIGN KEY (supplier_id) REFERENCES purchasing.suppliers(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--
