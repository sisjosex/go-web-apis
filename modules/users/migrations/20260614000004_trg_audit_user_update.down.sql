DROP TRIGGER IF EXISTS trg_audit_user_update ON auth.users;
DROP FUNCTION IF EXISTS users.trg_fn_audit_user_update();
