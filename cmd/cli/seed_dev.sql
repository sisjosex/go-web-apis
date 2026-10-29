-- Dev accounts the emulator signs in with, on the dev tenant `mi-negocio` (MOBILE-004).
-- Run by `make seed-dev` against the .env.platform database; safe to run again. Does
-- nothing where `mi-negocio` does not exist — it is a dev tenant, made by hand.
--
-- mobile.guardian@test.local / Guardian123*  — role `portal`, guardian of Bruno Mamani (on
-- the morning route, so a pickup there is a push) and Lucas Prueba.
DO $$
DECLARE
    v_tenant UUID;
    v_user   UUID;
    v_rider  UUID;
BEGIN
    SELECT id INTO v_tenant FROM tenancy.tenants WHERE slug = 'mi-negocio';
    IF v_tenant IS NULL THEN
        RAISE NOTICE 'seed-dev: no tenant mi-negocio, nothing seeded';
        RETURN;
    END IF;

    SELECT id INTO v_user FROM auth.users WHERE LOWER(email) = 'mobile.guardian@test.local';
    IF v_user IS NULL THEN
        INSERT INTO auth.users (first_name, last_name, email, password)
        VALUES ('Tutor', 'Dev', 'mobile.guardian@test.local', crypt('Guardian123*', gen_salt('bf')))
        RETURNING id INTO v_user;
    ELSE
        UPDATE auth.users
        SET password = crypt('Guardian123*', gen_salt('bf')), is_active = TRUE, deleted_at = NULL
        WHERE id = v_user;
    END IF;

    INSERT INTO tenancy.tenant_users (tenant_id, user_id, role)
    VALUES (v_tenant, v_user, 'portal')
    ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = 'portal', is_active = TRUE;

    FOR v_rider IN
        SELECT r.id FROM tracking.riders r
        INNER JOIN tracking.organizations o ON o.id = r.organization_id
        WHERE o.tenant_id = v_tenant
          AND (r.first_name, r.last_name) IN (('Bruno', 'Mamani'), ('Lucas', 'Prueba'))
    LOOP
        IF NOT EXISTS (SELECT 1 FROM tracking.rider_contacts c WHERE c.rider_id = v_rider AND c.user_id = v_user) THEN
            INSERT INTO tracking.rider_contacts (rider_id, relation, name, email, user_id)
            VALUES (v_rider, 'guardian', 'Tutor Dev', 'mobile.guardian@test.local', v_user);
        END IF;
    END LOOP;
END $$;

-- mobile.driver@test.local / Driver123*  — role `driver`, the account of driver Ana Quispe (MOBILE-036):
-- the driver's day on the emulator. Ana was made by hand, like the tenant; without her nothing is linked.
DO $$
DECLARE
    v_tenant UUID;
    v_user   UUID;
BEGIN
    SELECT id INTO v_tenant FROM tenancy.tenants WHERE slug = 'mi-negocio';
    IF v_tenant IS NULL THEN
        RETURN;
    END IF;

    SELECT id INTO v_user FROM auth.users WHERE LOWER(email) = 'mobile.driver@test.local';
    IF v_user IS NULL THEN
        INSERT INTO auth.users (first_name, last_name, email, password)
        VALUES ('Ana', 'Quispe', 'mobile.driver@test.local', crypt('Driver123*', gen_salt('bf')))
        RETURNING id INTO v_user;
    ELSE
        UPDATE auth.users
        SET password = crypt('Driver123*', gen_salt('bf')), is_active = TRUE, deleted_at = NULL
        WHERE id = v_user;
    END IF;

    INSERT INTO tenancy.tenant_users (tenant_id, user_id, role)
    VALUES (v_tenant, v_user, 'driver')
    ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = 'driver', is_active = TRUE;

    UPDATE tracking.drivers
    SET user_id = v_user
    WHERE tenant_id = v_tenant AND first_name = 'Ana' AND last_name = 'Quispe';
END $$;

-- mobile.passenger@test.local / Passenger123*  — role `portal`, the `self` contact of employee Rubén Choque
-- (Minera Andina): the adult passenger's home and pass (MOBILE-039). Rubén was made by hand, like the tenant.
DO $$
DECLARE
    v_tenant UUID;
    v_user   UUID;
    v_rider  UUID;
BEGIN
    SELECT id INTO v_tenant FROM tenancy.tenants WHERE slug = 'mi-negocio';
    IF v_tenant IS NULL THEN
        RETURN;
    END IF;

    SELECT id INTO v_user FROM auth.users WHERE LOWER(email) = 'mobile.passenger@test.local';
    IF v_user IS NULL THEN
        INSERT INTO auth.users (first_name, last_name, email, password)
        VALUES ('Rubén', 'Choque', 'mobile.passenger@test.local', crypt('Passenger123*', gen_salt('bf')))
        RETURNING id INTO v_user;
    ELSE
        UPDATE auth.users
        SET password = crypt('Passenger123*', gen_salt('bf')), is_active = TRUE, deleted_at = NULL
        WHERE id = v_user;
    END IF;

    INSERT INTO tenancy.tenant_users (tenant_id, user_id, role)
    VALUES (v_tenant, v_user, 'portal')
    ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = 'portal', is_active = TRUE;

    SELECT r.id INTO v_rider FROM tracking.riders r
    INNER JOIN tracking.organizations o ON o.id = r.organization_id
    WHERE o.tenant_id = v_tenant AND r.first_name = 'Rubén' AND r.last_name = 'Choque'
    LIMIT 1;
    IF v_rider IS NOT NULL AND NOT EXISTS (
        SELECT 1 FROM tracking.rider_contacts c WHERE c.rider_id = v_rider AND c.user_id = v_user
    ) THEN
        INSERT INTO tracking.rider_contacts (rider_id, relation, name, email, user_id)
        VALUES (v_rider, 'self', 'Rubén Choque', 'mobile.passenger@test.local', v_user);
    END IF;
END $$;

-- mobile.school@test.local   / School123*    — role `organization`, admin of Colegio Santa Rosa.
-- mobile.employer@test.local / Employer123*  — role `organization`, admin of Minera Andina S.A.
-- An organization that does not exist is skipped: both were made by hand, like the tenant.
DO $$
DECLARE
    v_tenant  UUID;
    v_user    UUID;
    v_org     UUID;
    v_account RECORD;
BEGIN
    SELECT id INTO v_tenant FROM tenancy.tenants WHERE slug = 'mi-negocio';
    IF v_tenant IS NULL THEN
        RETURN;
    END IF;

    FOR v_account IN
        SELECT * FROM (VALUES
            ('mobile.school@test.local', 'School123*', 'Colegio', 'Dev', 'Colegio Santa Rosa'),
            ('mobile.employer@test.local', 'Employer123*', 'Empleador', 'Dev', 'Minera Andina S.A.')
        ) AS a(email, password, first_name, last_name, organization)
    LOOP
        SELECT id INTO v_org FROM tracking.organizations
        WHERE tenant_id = v_tenant AND name = v_account.organization;
        IF v_org IS NULL THEN
            RAISE NOTICE 'seed-dev: no organization %, % not seeded', v_account.organization, v_account.email;
            CONTINUE;
        END IF;

        SELECT id INTO v_user FROM auth.users WHERE LOWER(email) = v_account.email;
        IF v_user IS NULL THEN
            INSERT INTO auth.users (first_name, last_name, email, password)
            VALUES (v_account.first_name, v_account.last_name, v_account.email,
                    crypt(v_account.password, gen_salt('bf')))
            RETURNING id INTO v_user;
        ELSE
            UPDATE auth.users
            SET password = crypt(v_account.password, gen_salt('bf')), is_active = TRUE, deleted_at = NULL
            WHERE id = v_user;
        END IF;

        INSERT INTO tenancy.tenant_users (tenant_id, user_id, role)
        VALUES (v_tenant, v_user, 'organization')
        ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = 'organization', is_active = TRUE;

        INSERT INTO tracking.organization_members (organization_id, user_id, role)
        VALUES (v_org, v_user, 'admin')
        ON CONFLICT (organization_id, user_id) DO UPDATE SET role = 'admin';
    END LOOP;
END $$;

-- web.noreports@test.local / NoReports123*  — role `member` with "Operador sin reportes": runs
-- the transport (routes, riders, clients, vehicles) without `tracking:reports:read`, so the
-- Reportes nav item stays hidden (TRACK-078).
DO $$
DECLARE
    v_tenant UUID;
    v_user   UUID;
    v_role   UUID;
BEGIN
    SELECT id INTO v_tenant FROM tenancy.tenants WHERE slug = 'mi-negocio';
    IF v_tenant IS NULL THEN
        RETURN;
    END IF;

    INSERT INTO tenancy.roles (tenant_id, name, description)
    VALUES (v_tenant, 'Operador sin reportes', 'Dev: operates taypi go without the reports')
    ON CONFLICT (tenant_id, name) DO UPDATE SET description = EXCLUDED.description
    RETURNING id INTO v_role;

    DELETE FROM tenancy.role_permissions WHERE role_id = v_role;
    INSERT INTO tenancy.role_permissions (role_id, permission_code)
    SELECT v_role, UNNEST(ARRAY[
        'tracking:routes:read', 'tracking:riders:read', 'tracking:organizations:read', 'tracking:vehicles:read'
    ]);

    SELECT id INTO v_user FROM auth.users WHERE LOWER(email) = 'web.noreports@test.local';
    IF v_user IS NULL THEN
        INSERT INTO auth.users (first_name, last_name, email, password)
        VALUES ('Operador', 'Sin Reportes', 'web.noreports@test.local', crypt('NoReports123*', gen_salt('bf')))
        RETURNING id INTO v_user;
    ELSE
        UPDATE auth.users
        SET password = crypt('NoReports123*', gen_salt('bf')), is_active = TRUE, deleted_at = NULL
        WHERE id = v_user;
    END IF;

    INSERT INTO tenancy.tenant_users (tenant_id, user_id, role)
    VALUES (v_tenant, v_user, 'member')
    ON CONFLICT (tenant_id, user_id) DO UPDATE SET role = 'member', is_active = TRUE;

    -- Assigned by the tenant's owner, as the roles screen would.
    INSERT INTO tenancy.user_roles (user_id, role_id, tenant_id, assigned_by)
    SELECT v_user, v_role, v_tenant, tu.user_id
    FROM tenancy.tenant_users tu
    WHERE tu.tenant_id = v_tenant AND tu.role = 'owner'
    LIMIT 1
    ON CONFLICT (user_id, role_id) DO NOTHING;
END $$;
