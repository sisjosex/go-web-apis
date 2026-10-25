-- TENANCY-006 D2: the business types a workspace picked, and the module it opens on (APP-015 D2).
ALTER TABLE tenancy.tenants
    ADD COLUMN verticals varchar(32)[] NOT NULL DEFAULT '{}',
    ADD COLUMN primary_module varchar(32) NULL;
