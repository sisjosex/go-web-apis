# Cómo Crear Nuevas Migraciones

Este proyecto usa un sistema de migraciones **modular**. Cada módulo (`core`, `auth`, `users`) tiene sus propias migraciones.

## 🚀 Crear Nueva Migración (Cross-Platform)

### Comando único (funciona en Windows, Linux, Mac):

```bash
go run cmd/migration/main.go -module=<MODULE> -name=<NOMBRE>
```

**Ejemplos:**
```bash
# Agregar tabla de refresh tokens al módulo auth
go run cmd/migration/main.go -module=auth -name=add_refresh_tokens

# Agregar campo avatar al módulo users
go run cmd/migration/main.go -module=users -name=add_avatar_field

# Agregar extensión PostGIS al módulo core
go run cmd/migration/main.go -module=core -name=add_postgis_extension
```

**Salida:**
```
🎉 Migration created successfully!
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
📦 Module:    auth
📝 Name:      add_refresh_tokens
🕐 Timestamp: 20251129084818

📁 Files created:
   ↑ modules/auth/migrations/20251129084818_add_refresh_tokens.up.sql
   ↓ modules/auth/migrations/20251129084818_add_refresh_tokens.down.sql

📝 Next steps:
   1. Edit 20251129084818_add_refresh_tokens.up.sql to add your SQL
   2. Edit 20251129084818_add_refresh_tokens.down.sql for rollback
   3. Run: go run .
```

## 📝 Después de Crear los Archivos

1. **Edita el archivo `.up.sql`** con tu migración:
```sql
-- modules/auth/migrations/20251129084500_add_refresh_tokens.up.sql
CREATE TABLE IF NOT EXISTS auth.refresh_tokens (
    id UUID PRIMARY KEY DEFAULT public.uuid_generate_v4(),
    user_id UUID NOT NULL REFERENCES auth.users(id) ON DELETE CASCADE,
    token TEXT NOT NULL,
    expires_at TIMESTAMP WITH TIME ZONE NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX idx_refresh_tokens_user_id ON auth.refresh_tokens(user_id);
```

2. **Edita el archivo `.down.sql`** con el rollback:
```sql
-- modules/auth/migrations/20251129084500_add_refresh_tokens.down.sql
DROP TABLE IF EXISTS auth.refresh_tokens;
```

3. **Ejecuta el servidor** y la migración se aplicará automáticamente:
```bash
go run .
```

## 📦 Módulos Disponibles

- **`core`**: Extensiones, schemas, funciones compartidas
- **`auth`**: Tablas y SPs de autenticación (users, sessions, tokens)
- **`users`**: Tablas y SPs de gestión de usuarios (CRUD admin)

## 🔍 Ver Estado de Migraciones

Al iniciar el servidor, verás:
```
📋 Module Status:
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
✅ core            | Status: active   | Migrations: 1
✅ auth            | Status: active   | Migrations: 9  ← +1 nueva
✅ users           | Status: active   | Migrations: 6
━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━━
```

## 📚 Estructura de Archivos

```
modules/
├── core/
│   └── migrations/
│       ├── 20240922000001_init_extensions.up.sql
│       └── 20240922000001_init_extensions.down.sql
├── auth/
│   └── migrations/
│       ├── 20240922230933_table_users.up.sql
│       ├── 20240922230933_table_users.down.sql
│       └── ... (8 migraciones)
└── users/
    └── migrations/
        ├── 20240922231132_sp_create_user.up.sql
        ├── 20240922231132_sp_create_user.down.sql
        └── ... (6 migraciones)
```

## ⚡ Tips

- ✅ Usa timestamps reales (YYYYMMDDHHMMSS)
- ✅ Nombres descriptivos (`add_refresh_tokens`, no `migration1`)
- ✅ Siempre crea el `.down.sql` para rollback
- ✅ Usa `IF NOT EXISTS` / `IF EXISTS` para idempotencia
- ✅ Prueba tu migración antes de commitear
