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
