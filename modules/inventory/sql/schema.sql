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
-- Name: inventory; Type: SCHEMA; Schema: -; Owner: -
--

CREATE SCHEMA inventory;


--
-- Name: batch_movements; Type: TABLE; Schema: inventory; Owner: -
--

CREATE TABLE inventory.batch_movements (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    batch_id uuid NOT NULL,
    movement_id uuid,
    quantity_consumed numeric(10,2) NOT NULL,
    cost_of_goods_sold numeric(10,2) NOT NULL,
    movement_date timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: inventory_movements; Type: TABLE; Schema: inventory; Owner: -
--

CREATE TABLE inventory.inventory_movements (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id uuid NOT NULL,
    product_id uuid NOT NULL,
    movement_type character varying(50) NOT NULL,
    quantity numeric(12,2) NOT NULL,
    reference_type character varying(50),
    reference_id uuid,
    unit_cost numeric(12,2),
    notes text,
    created_by uuid,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    batch_id uuid,
    sku_id uuid NOT NULL,
    CONSTRAINT inventory_movements_movement_type_check CHECK (((movement_type)::text = ANY ((ARRAY['PURCHASE'::character varying, 'SALE'::character varying, 'ADJUSTMENT'::character varying, 'TRANSFER'::character varying, 'RETURN'::character varying, 'WASTE'::character varying, 'PRODUCTION'::character varying])::text[])))
);


--
-- Name: COLUMN inventory_movements.sku_id; Type: COMMENT; Schema: inventory; Owner: -
--

COMMENT ON COLUMN inventory.inventory_movements.sku_id IS 'The SKU this movement applied to; the product default until phase 2 generates real combinations.';


--
-- Name: product_batches; Type: TABLE; Schema: inventory; Owner: -
--

CREATE TABLE inventory.product_batches (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    product_id uuid NOT NULL,
    lot_number character varying(100) NOT NULL,
    purchase_date date NOT NULL,
    expiry_date date NOT NULL,
    unit_cost numeric(10,2) NOT NULL,
    initial_quantity numeric(10,2) NOT NULL,
    current_quantity numeric(10,2) NOT NULL,
    status character varying(50) DEFAULT 'active'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    sku_id uuid NOT NULL
);


--
-- Name: COLUMN product_batches.sku_id; Type: COMMENT; Schema: inventory; Owner: -
--

COMMENT ON COLUMN inventory.product_batches.sku_id IS 'The SKU this batch belongs to; the product default until phase 2 generates real combinations.';


--
-- Name: product_categories; Type: TABLE; Schema: inventory; Owner: -
--

CREATE TABLE inventory.product_categories (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    parent_id uuid,
    name character varying(255) NOT NULL,
    slug character varying(255) NOT NULL,
    description text,
    icon_url character varying(512),
    display_order integer DEFAULT 0,
    is_active boolean DEFAULT true,
    deleted_at timestamp without time zone,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    updated_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: product_category_mapping; Type: TABLE; Schema: inventory; Owner: -
--

CREATE TABLE inventory.product_category_mapping (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    product_id uuid NOT NULL,
    category_id uuid NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL
);


--
-- Name: product_media; Type: TABLE; Schema: inventory; Owner: -
--

CREATE TABLE inventory.product_media (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    product_id uuid NOT NULL,
    variant_option_id uuid,
    media_type character varying(10) NOT NULL,
    url text NOT NULL,
    alt_text character varying(255),
    is_primary boolean DEFAULT false NOT NULL,
    sort_order integer DEFAULT 0 NOT NULL,
    created_at timestamp without time zone DEFAULT now() NOT NULL,
    CONSTRAINT product_media_media_type_check CHECK (((media_type)::text = ANY ((ARRAY['image'::character varying, 'video'::character varying])::text[])))
);


--
-- Name: product_sku_options; Type: TABLE; Schema: inventory; Owner: -
--

CREATE TABLE inventory.product_sku_options (
    sku_id uuid NOT NULL,
    variant_group_id uuid NOT NULL,
    option_id uuid NOT NULL,
    product_id uuid NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL
);


--
-- Name: TABLE product_sku_options; Type: COMMENT; Schema: inventory; Owner: -
--

COMMENT ON TABLE inventory.product_sku_options IS 'One row per axis of a combination: which option of each affects_inventory group this SKU is made of. product_id is carried so the composite FKs can reject an option from another group and a group from another product.';


--
-- Name: product_skus; Type: TABLE; Schema: inventory; Owner: -
--

CREATE TABLE inventory.product_skus (
    id uuid DEFAULT gen_random_uuid() NOT NULL,
    tenant_id uuid NOT NULL,
    product_id uuid NOT NULL,
    sku character varying(50) NOT NULL,
    price_modifier numeric(12,2) DEFAULT 0 NOT NULL,
    combination_key text DEFAULT ''::text NOT NULL,
    is_default boolean DEFAULT false NOT NULL,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP NOT NULL,
    CONSTRAINT product_skus_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'discontinued'::character varying])::text[])))
);


--
-- Name: TABLE product_skus; Type: COMMENT; Schema: inventory; Owner: -
--

COMMENT ON TABLE inventory.product_skus IS 'Sellable combination of a product (Polera-M, Polera-XL); the only entity stock, movements and batches hang off. NOT the definition of the options a product offers - that is inventory.product_variant_groups / inventory.product_variant_options.';


--
-- Name: COLUMN product_skus.price_modifier; Type: COMMENT; Schema: inventory; Owner: -
--

COMMENT ON COLUMN inventory.product_skus.price_modifier IS 'Authoritative delta over products.base_price for anything sold by SKU (INV-007 D3). Derived, never edited: it is the sum of the price_modifier of the options composing the combination, maintained by inventory.fn_recalc_sku_price_from_options (INV-012 D1). 0 on the default SKU, which composes no option.';


--
-- Name: COLUMN product_skus.combination_key; Type: COMMENT; Schema: inventory; Owner: -
--

COMMENT ON COLUMN inventory.product_skus.combination_key IS 'The combination''s option UUIDs, sorted ascending as text, joined by '':''. Empty string for the default SKU. Format fixed by INV-007 D4.';


--
-- Name: COLUMN product_skus.is_default; Type: COMMENT; Schema: inventory; Owner: -
--

COMMENT ON COLUMN inventory.product_skus.is_default IS 'The product''s unassigned bucket: created by the backfill, never deleted, and from phase 2 on it holds the quantity not yet redistributed across real combinations.';


--
-- Name: product_stock; Type: TABLE; Schema: inventory; Owner: -
--

CREATE TABLE inventory.product_stock (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    product_id uuid NOT NULL,
    current_quantity numeric(12,2) DEFAULT 0 NOT NULL,
    reserved_quantity numeric(12,2) DEFAULT 0 NOT NULL,
    reorder_level numeric(12,2) DEFAULT 10 NOT NULL,
    status character varying(50) DEFAULT 'ok'::character varying NOT NULL,
    last_updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    sku_id uuid NOT NULL
);


--
-- Name: COLUMN product_stock.sku_id; Type: COMMENT; Schema: inventory; Owner: -
--

COMMENT ON COLUMN inventory.product_stock.sku_id IS 'The SKU these quantities belong to. product_id is kept alongside it and the composite FK guarantees the two agree.';


--
-- Name: product_variant_groups; Type: TABLE; Schema: inventory; Owner: -
--

CREATE TABLE inventory.product_variant_groups (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    product_id uuid NOT NULL,
    group_type character varying(50) NOT NULL,
    is_required boolean DEFAULT false,
    max_selections integer DEFAULT 1,
    sort_order integer DEFAULT 0,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    affects_inventory boolean DEFAULT false NOT NULL,
    CONSTRAINT chk_variant_groups_axis_single_choice CHECK (((affects_inventory = false) OR ((max_selections = 1) AND (is_required = true))))
);


--
-- Name: COLUMN product_variant_groups.affects_inventory; Type: COMMENT; Schema: inventory; Owner: -
--

COMMENT ON COLUMN inventory.product_variant_groups.affects_inventory IS 'TRUE = this group is an inventory axis: every one of its options multiplies the product''s SKUs. FALSE (the default, and every pre-INV-007 row) = a menu modifier that changes price and presentation but never stock.';


--
-- Name: product_variant_options; Type: TABLE; Schema: inventory; Owner: -
--

CREATE TABLE inventory.product_variant_options (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    variant_group_id uuid NOT NULL,
    option_name character varying(255) NOT NULL,
    price_modifier numeric(12,2) DEFAULT 0,
    is_available boolean DEFAULT true,
    sort_order integer DEFAULT 0,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP
);


--
-- Name: TABLE product_variant_options; Type: COMMENT; Schema: inventory; Owner: -
--

COMMENT ON TABLE inventory.product_variant_options IS 'The DEFINITION of one option a product offers (Vanilla, XL, Extra cheese). NOT a sellable combination and NOT a stock-bearing entity - that is inventory.product_skus.';


--
-- Name: products; Type: TABLE; Schema: inventory; Owner: -
--

CREATE TABLE inventory.products (
    id uuid DEFAULT public.uuid_generate_v4() NOT NULL,
    tenant_id uuid NOT NULL,
    sku character varying(50) NOT NULL,
    name character varying(255) NOT NULL,
    description text,
    category character varying(50),
    base_price numeric(12,2) NOT NULL,
    has_variants boolean DEFAULT false,
    status character varying(20) DEFAULT 'active'::character varying NOT NULL,
    created_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    updated_at timestamp without time zone DEFAULT CURRENT_TIMESTAMP,
    stock_by_variant boolean DEFAULT false NOT NULL,
    CONSTRAINT products_status_check CHECK (((status)::text = ANY ((ARRAY['active'::character varying, 'discontinued'::character varying])::text[])))
);


--
-- Name: COLUMN products.stock_by_variant; Type: COMMENT; Schema: inventory; Owner: -
--

COMMENT ON COLUMN inventory.products.stock_by_variant IS 'TRUE when the product has at least one affects_inventory variant group, so its stock lives on generated combinations rather than on the default SKU alone. Written by inventory.sp_generate_product_skus, which owns it, and cleared by inventory.sp_update_product_with_variants when the last axis goes away. Never set by hand.';


--
-- Name: v_product_stock_totals; Type: VIEW; Schema: inventory; Owner: -
--

CREATE VIEW inventory.v_product_stock_totals AS
 SELECT product_id,
    sum(current_quantity) AS current_quantity,
    sum(reserved_quantity) AS reserved_quantity,
    sum((current_quantity - reserved_quantity)) AS available_quantity,
    sum(reorder_level) AS reorder_level,
    inventory.fn_stock_status(sum(current_quantity), sum(reorder_level)) AS status,
    max(last_updated_at) AS last_updated_at
   FROM inventory.product_stock ps
  GROUP BY product_id;


--
-- Name: VIEW v_product_stock_totals; Type: COMMENT; Schema: inventory; Owner: -
--

COMMENT ON VIEW inventory.v_product_stock_totals IS 'Per-product sums over the product SKUs. status is recomputed through inventory.fn_stock_status so the product-level answer can never contradict the per-SKU rows it sums.';


--
-- Name: batch_movements batch_movements_pkey; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.batch_movements
    ADD CONSTRAINT batch_movements_pkey PRIMARY KEY (id);


--
-- Name: inventory_movements inventory_movements_pkey; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.inventory_movements
    ADD CONSTRAINT inventory_movements_pkey PRIMARY KEY (id);


--
-- Name: product_batches product_batches_pkey; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_batches
    ADD CONSTRAINT product_batches_pkey PRIMARY KEY (id);


--
-- Name: product_categories product_categories_pkey; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_categories
    ADD CONSTRAINT product_categories_pkey PRIMARY KEY (id);


--
-- Name: product_category_mapping product_category_mapping_pkey; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_category_mapping
    ADD CONSTRAINT product_category_mapping_pkey PRIMARY KEY (id);


--
-- Name: product_category_mapping product_category_mapping_product_id_category_id_key; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_category_mapping
    ADD CONSTRAINT product_category_mapping_product_id_category_id_key UNIQUE (product_id, category_id);


--
-- Name: product_media product_media_pkey; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_media
    ADD CONSTRAINT product_media_pkey PRIMARY KEY (id);


--
-- Name: product_sku_options product_sku_options_pkey; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_sku_options
    ADD CONSTRAINT product_sku_options_pkey PRIMARY KEY (sku_id, variant_group_id);


--
-- Name: product_skus product_skus_pkey; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_skus
    ADD CONSTRAINT product_skus_pkey PRIMARY KEY (id);


--
-- Name: product_skus product_skus_product_combination_key; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_skus
    ADD CONSTRAINT product_skus_product_combination_key UNIQUE (product_id, combination_key);


--
-- Name: product_skus product_skus_tenant_sku_key; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_skus
    ADD CONSTRAINT product_skus_tenant_sku_key UNIQUE (tenant_id, sku);


--
-- Name: product_stock product_stock_pkey; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_stock
    ADD CONSTRAINT product_stock_pkey PRIMARY KEY (id);


--
-- Name: product_stock product_stock_sku_id_key; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_stock
    ADD CONSTRAINT product_stock_sku_id_key UNIQUE (sku_id);


--
-- Name: product_variant_groups product_variant_groups_pkey; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_variant_groups
    ADD CONSTRAINT product_variant_groups_pkey PRIMARY KEY (id);


--
-- Name: product_variant_options product_variant_options_pkey; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_variant_options
    ADD CONSTRAINT product_variant_options_pkey PRIMARY KEY (id);


--
-- Name: products products_pkey; Type: CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.products
    ADD CONSTRAINT products_pkey PRIMARY KEY (id);


--
-- Name: idx_batch_movements_batch_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_batch_movements_batch_id ON inventory.batch_movements USING btree (batch_id);


--
-- Name: idx_batch_movements_movement_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_batch_movements_movement_id ON inventory.batch_movements USING btree (movement_id);


--
-- Name: idx_batch_movements_tenant_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_batch_movements_tenant_id ON inventory.batch_movements USING btree (tenant_id);


--
-- Name: idx_inventory_movements_batch_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_inventory_movements_batch_id ON inventory.inventory_movements USING btree (batch_id);


--
-- Name: idx_movements_created_at; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_movements_created_at ON inventory.inventory_movements USING btree (created_at);


--
-- Name: idx_movements_product; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_movements_product ON inventory.inventory_movements USING btree (product_id);


--
-- Name: idx_movements_reference; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_movements_reference ON inventory.inventory_movements USING btree (reference_type, reference_id);


--
-- Name: idx_movements_sku_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_movements_sku_id ON inventory.inventory_movements USING btree (sku_id);


--
-- Name: idx_movements_tenant_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_movements_tenant_id ON inventory.inventory_movements USING btree (tenant_id);


--
-- Name: idx_movements_type; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_movements_type ON inventory.inventory_movements USING btree (movement_type);


--
-- Name: idx_product_batches_expiry_date; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_batches_expiry_date ON inventory.product_batches USING btree (expiry_date);


--
-- Name: idx_product_batches_id_sku_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE UNIQUE INDEX idx_product_batches_id_sku_id ON inventory.product_batches USING btree (id, sku_id);


--
-- Name: idx_product_batches_product_expiry; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_batches_product_expiry ON inventory.product_batches USING btree (product_id, expiry_date);


--
-- Name: idx_product_batches_product_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_batches_product_id ON inventory.product_batches USING btree (product_id);


--
-- Name: idx_product_batches_sku_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_batches_sku_id ON inventory.product_batches USING btree (sku_id);


--
-- Name: idx_product_batches_tenant_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_batches_tenant_id ON inventory.product_batches USING btree (tenant_id);


--
-- Name: idx_product_batches_tenant_open_expiry; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_batches_tenant_open_expiry ON inventory.product_batches USING btree (tenant_id, expiry_date) WHERE ((status)::text = ANY ((ARRAY['active'::character varying, 'expiring_soon'::character varying])::text[]));


--
-- Name: idx_product_categories_active; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_categories_active ON inventory.product_categories USING btree (id) WHERE (deleted_at IS NULL);


--
-- Name: idx_product_categories_description_search_gin; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_categories_description_search_gin ON inventory.product_categories USING gin (lower(COALESCE(description, ''::text)) public.gin_trgm_ops) WHERE (deleted_at IS NULL);


--
-- Name: idx_product_categories_hierarchy_active; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_categories_hierarchy_active ON inventory.product_categories USING btree (parent_id, display_order) WHERE (deleted_at IS NULL);


--
-- Name: idx_product_categories_is_active; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_categories_is_active ON inventory.product_categories USING btree (is_active);


--
-- Name: idx_product_categories_parent_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_categories_parent_id ON inventory.product_categories USING btree (parent_id);


--
-- Name: idx_product_categories_search_gin; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_categories_search_gin ON inventory.product_categories USING gin ((((lower((name)::text) || ' '::text) || lower((COALESCE(slug, ''::character varying))::text))) public.gin_trgm_ops) WHERE (deleted_at IS NULL);


--
-- Name: idx_product_categories_slug; Type: INDEX; Schema: inventory; Owner: -
--

CREATE UNIQUE INDEX idx_product_categories_slug ON inventory.product_categories USING btree (tenant_id, slug);


--
-- Name: idx_product_categories_status_active; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_categories_status_active ON inventory.product_categories USING btree (is_active) WHERE ((deleted_at IS NULL) AND (is_active = true));


--
-- Name: idx_product_categories_tenant_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_categories_tenant_id ON inventory.product_categories USING btree (tenant_id);


--
-- Name: idx_product_category_mapping_category_fast; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_category_mapping_category_fast ON inventory.product_category_mapping USING btree (category_id, product_id);


--
-- Name: idx_product_category_mapping_product_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_category_mapping_product_id ON inventory.product_category_mapping USING btree (product_id);


--
-- Name: idx_product_media_product_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_media_product_id ON inventory.product_media USING btree (product_id);


--
-- Name: idx_product_media_tenant_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_media_tenant_id ON inventory.product_media USING btree (tenant_id);


--
-- Name: idx_product_media_variant_option_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_media_variant_option_id ON inventory.product_media USING btree (variant_option_id);


--
-- Name: idx_product_sku_options_option_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_sku_options_option_id ON inventory.product_sku_options USING btree (option_id);


--
-- Name: idx_product_sku_options_product_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_sku_options_product_id ON inventory.product_sku_options USING btree (product_id);


--
-- Name: idx_product_skus_id_product_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE UNIQUE INDEX idx_product_skus_id_product_id ON inventory.product_skus USING btree (id, product_id);


--
-- Name: idx_product_skus_one_default_per_product; Type: INDEX; Schema: inventory; Owner: -
--

CREATE UNIQUE INDEX idx_product_skus_one_default_per_product ON inventory.product_skus USING btree (product_id) WHERE is_default;


--
-- Name: idx_product_skus_product_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_skus_product_id ON inventory.product_skus USING btree (product_id);


--
-- Name: idx_product_skus_tenant_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_skus_tenant_id ON inventory.product_skus USING btree (tenant_id);


--
-- Name: idx_product_stock_product; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_stock_product ON inventory.product_stock USING btree (product_id);


--
-- Name: idx_product_stock_sku_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_product_stock_sku_id ON inventory.product_stock USING btree (sku_id);


--
-- Name: idx_products_name_trgm; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_products_name_trgm ON inventory.products USING gin (name public.gin_trgm_ops);


--
-- Name: idx_products_sku; Type: INDEX; Schema: inventory; Owner: -
--

CREATE UNIQUE INDEX idx_products_sku ON inventory.products USING btree (tenant_id, sku);


--
-- Name: idx_products_sku_trgm; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_products_sku_trgm ON inventory.products USING gin (sku public.gin_trgm_ops);


--
-- Name: idx_products_status; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_products_status ON inventory.products USING btree (status);


--
-- Name: idx_products_tenant_active_created; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_products_tenant_active_created ON inventory.products USING btree (tenant_id, created_at DESC, id DESC) WHERE ((status)::text = 'active'::text);


--
-- Name: idx_products_tenant_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_products_tenant_id ON inventory.products USING btree (tenant_id);


--
-- Name: idx_variant_groups_id_product_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE UNIQUE INDEX idx_variant_groups_id_product_id ON inventory.product_variant_groups USING btree (id, product_id);


--
-- Name: idx_variant_groups_product; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_variant_groups_product ON inventory.product_variant_groups USING btree (product_id);


--
-- Name: idx_variant_groups_product_type; Type: INDEX; Schema: inventory; Owner: -
--

CREATE UNIQUE INDEX idx_variant_groups_product_type ON inventory.product_variant_groups USING btree (product_id, group_type);


--
-- Name: idx_variant_options_group; Type: INDEX; Schema: inventory; Owner: -
--

CREATE INDEX idx_variant_options_group ON inventory.product_variant_options USING btree (variant_group_id);


--
-- Name: idx_variant_options_group_name; Type: INDEX; Schema: inventory; Owner: -
--

CREATE UNIQUE INDEX idx_variant_options_group_name ON inventory.product_variant_options USING btree (variant_group_id, option_name);


--
-- Name: idx_variant_options_id_group_id; Type: INDEX; Schema: inventory; Owner: -
--

CREATE UNIQUE INDEX idx_variant_options_id_group_id ON inventory.product_variant_options USING btree (id, variant_group_id);


--
-- Name: product_sku_options trg_sku_options_combination_key; Type: TRIGGER; Schema: inventory; Owner: -
--

CREATE CONSTRAINT TRIGGER trg_sku_options_combination_key AFTER INSERT OR DELETE OR UPDATE ON inventory.product_sku_options DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION inventory.fn_check_sku_combination_key();


--
-- Name: product_sku_options trg_sku_options_price_from_options; Type: TRIGGER; Schema: inventory; Owner: -
--

CREATE TRIGGER trg_sku_options_price_from_options AFTER INSERT ON inventory.product_sku_options REFERENCING NEW TABLE AS new_rows FOR EACH STATEMENT EXECUTE FUNCTION inventory.fn_recalc_sku_price_from_options();


--
-- Name: product_variant_options trg_variant_options_price_to_skus; Type: TRIGGER; Schema: inventory; Owner: -
--

CREATE TRIGGER trg_variant_options_price_to_skus AFTER UPDATE ON inventory.product_variant_options REFERENCING OLD TABLE AS old_rows NEW TABLE AS new_rows FOR EACH STATEMENT EXECUTE FUNCTION inventory.fn_recalc_sku_price_from_options();


--
-- Name: batch_movements batch_movements_batch_id_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.batch_movements
    ADD CONSTRAINT batch_movements_batch_id_fkey FOREIGN KEY (batch_id) REFERENCES inventory.product_batches(id) ON DELETE CASCADE;


--
-- Name: batch_movements batch_movements_movement_id_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.batch_movements
    ADD CONSTRAINT batch_movements_movement_id_fkey FOREIGN KEY (movement_id) REFERENCES inventory.inventory_movements(id) ON DELETE SET NULL;


--
-- Name: inventory_movements inventory_movements_batch_id_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.inventory_movements
    ADD CONSTRAINT inventory_movements_batch_id_fkey FOREIGN KEY (batch_id) REFERENCES inventory.product_batches(id) ON DELETE SET NULL;


--
-- Name: inventory_movements inventory_movements_product_id_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.inventory_movements
    ADD CONSTRAINT inventory_movements_product_id_fkey FOREIGN KEY (product_id) REFERENCES inventory.products(id);


--
-- Name: inventory_movements inventory_movements_sku_product_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.inventory_movements
    ADD CONSTRAINT inventory_movements_sku_product_fkey FOREIGN KEY (sku_id, product_id) REFERENCES inventory.product_skus(id, product_id);


--
-- Name: product_batches product_batches_product_id_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_batches
    ADD CONSTRAINT product_batches_product_id_fkey FOREIGN KEY (product_id) REFERENCES inventory.products(id) ON DELETE CASCADE;


--
-- Name: product_batches product_batches_sku_product_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_batches
    ADD CONSTRAINT product_batches_sku_product_fkey FOREIGN KEY (sku_id, product_id) REFERENCES inventory.product_skus(id, product_id) ON DELETE CASCADE;


--
-- Name: product_categories product_categories_parent_id_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_categories
    ADD CONSTRAINT product_categories_parent_id_fkey FOREIGN KEY (parent_id) REFERENCES inventory.product_categories(id) ON DELETE SET NULL;


--
-- Name: product_category_mapping product_category_mapping_category_id_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_category_mapping
    ADD CONSTRAINT product_category_mapping_category_id_fkey FOREIGN KEY (category_id) REFERENCES inventory.product_categories(id) ON DELETE CASCADE;


--
-- Name: product_category_mapping product_category_mapping_product_id_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_category_mapping
    ADD CONSTRAINT product_category_mapping_product_id_fkey FOREIGN KEY (product_id) REFERENCES inventory.products(id) ON DELETE CASCADE;


--
-- Name: product_media product_media_product_id_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_media
    ADD CONSTRAINT product_media_product_id_fkey FOREIGN KEY (product_id) REFERENCES inventory.products(id) ON DELETE CASCADE;


--
-- Name: product_media product_media_variant_option_id_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_media
    ADD CONSTRAINT product_media_variant_option_id_fkey FOREIGN KEY (variant_option_id) REFERENCES inventory.product_variant_options(id) ON DELETE CASCADE;


--
-- Name: product_sku_options product_sku_options_group_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_sku_options
    ADD CONSTRAINT product_sku_options_group_fkey FOREIGN KEY (variant_group_id, product_id) REFERENCES inventory.product_variant_groups(id, product_id) ON DELETE CASCADE;


--
-- Name: product_sku_options product_sku_options_option_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_sku_options
    ADD CONSTRAINT product_sku_options_option_fkey FOREIGN KEY (option_id, variant_group_id) REFERENCES inventory.product_variant_options(id, variant_group_id) ON DELETE CASCADE;


--
-- Name: product_sku_options product_sku_options_sku_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_sku_options
    ADD CONSTRAINT product_sku_options_sku_fkey FOREIGN KEY (sku_id, product_id) REFERENCES inventory.product_skus(id, product_id) ON DELETE CASCADE;


--
-- Name: product_skus product_skus_product_id_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_skus
    ADD CONSTRAINT product_skus_product_id_fkey FOREIGN KEY (product_id) REFERENCES inventory.products(id) ON DELETE CASCADE;


--
-- Name: product_stock product_stock_product_id_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_stock
    ADD CONSTRAINT product_stock_product_id_fkey FOREIGN KEY (product_id) REFERENCES inventory.products(id) ON DELETE CASCADE;


--
-- Name: product_stock product_stock_sku_product_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_stock
    ADD CONSTRAINT product_stock_sku_product_fkey FOREIGN KEY (sku_id, product_id) REFERENCES inventory.product_skus(id, product_id) ON DELETE CASCADE;


--
-- Name: product_variant_groups product_variant_groups_product_id_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_variant_groups
    ADD CONSTRAINT product_variant_groups_product_id_fkey FOREIGN KEY (product_id) REFERENCES inventory.products(id) ON DELETE CASCADE;


--
-- Name: product_variant_options product_variant_options_variant_group_id_fkey; Type: FK CONSTRAINT; Schema: inventory; Owner: -
--

ALTER TABLE ONLY inventory.product_variant_options
    ADD CONSTRAINT product_variant_options_variant_group_id_fkey FOREIGN KEY (variant_group_id) REFERENCES inventory.product_variant_groups(id) ON DELETE CASCADE;


--
-- PostgreSQL database dump complete
--
