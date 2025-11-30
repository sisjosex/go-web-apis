# Módulo de Multitenancy

Sistema completo de multitenancy configurable y opcional para el proyecto Go Web API.

## 📋 Características

- ✅ **Arquitectura flexible**: Soporta 1 Tenant = 1 Database o Schema-based tenancy
- ✅ **Autenticación centralizada**: Usuarios globales en Main DB, acceso multi-tenant
- ✅ **Completamente opcional**: Habilitar/deshabilitar via configuración
- ✅ **Seguro**: Verificación de acceso usuario-tenant con roles
- ✅ **Stored Procedures**: Toda la lógica en base de datos
- ✅ **Connection Pooling**: Pools separados y cacheados por tenant
- ✅ **Multi-idioma**: Errores traducidos (EN/ES)
- ✅ **RESTful API**: Endpoints siguiendo mejores prácticas
- ✅ **Documentación Swagger**: Auto-generada

## 🚀 Habilitación del Módulo

### 1. Configurar variables de entorno (.env)

```env
# Habilitar el módulo tenancy
ENABLED_MODULES=core,auth,users,tenancy

# Configuración de multitenancy
TENANCY_ENABLED=true
TENANCY_DATABASE_POOL_SIZE=5
TENANCY_ALLOW_CUSTOM_DATABASE_URLS=true
TENANCY_DEFAULT_SCHEMA=public
```

### 2. Ejecutar migraciones

Al iniciar la aplicación, las migraciones del módulo `tenancy` se ejecutarán automáticamente si está habilitado en `ENABLED_MODULES`.

```bash
go run .
```

Las siguientes tablas y stored procedures se crearán en el schema `tenancy`:

**Tablas:**
- `tenancy.tenants` - Información de tenants
- `tenancy.tenant_users` - Relación usuario-tenant con roles (FK a `auth.users`)

**Stored Procedures:**
- `tenancy.sp_create_tenant` - Crear nuevo tenant
- `tenancy.sp_get_tenant_by_slug` - Obtener tenant por slug
- `tenancy.sp_verify_user_tenant_access` - Verificar acceso de usuario
- `tenancy.sp_get_user_tenants` - Listar tenants del usuario
- `tenancy.sp_add_user_to_tenant` - Agregar usuario a tenant
- `tenancy.sp_remove_user_from_tenant` - Remover usuario de tenant
- `tenancy.sp_update_tenant` - Actualizar tenant

## 🔐 Estrategia de Autenticación (Centralizada)

### Single Sign-On Multi-Tenant

El sistema implementa **autenticación centralizada** donde:

**Main Database:**
- ✅ `auth.users` - TODOS los usuarios del sistema
- ✅ `auth.user_sessions` - Sesiones activas
- ✅ `tenancy.tenants` - Catálogo de tenants
- ✅ `tenancy.tenant_users` - Relación usuario ↔ tenant (roles)

**Tenant Database:**
- ✅ Solo datos de negocio (facturas, productos, inventario)
- ❌ NO tiene tabla `auth.users` (usuarios centralizados)
- ❌ NO tiene sesiones (autenticación en Main DB)

### Flujo de Login Multi-Tenant

```bash
# 1. Usuario se autentica contra Main DB (sin tenant)
POST /api/v1/auth/login
{
  "email": "jose@example.com",
  "password": "secret"
}

# Response:
{
  "access_token": "eyJhbGci...",  # Contiene: user_id + session_id
  "user": {
    "id": "uuid",
    "email": "jose@example.com"
  }
}

# 2. Usuario lista sus tenants disponibles
GET /api/v1/tenants/my-tenants
Authorization: Bearer eyJhbGci...

# Response:
[
  {"slug": "acme", "name": "Acme Corp", "role": "owner"},
  {"slug": "beta", "name": "Beta Inc", "role": "member"}
]

# 3. Usuario accede a recursos del tenant
GET /api/v1/tenants/acme/invoices
Authorization: Bearer eyJhbGci...

# Middleware verifica:
# - JWT válido (extrae user_id)
# - tenancy.sp_verify_user_tenant_access(user_id, 'acme')
# - Si OK: obtiene database_url del tenant
# - Ejecuta query en Tenant DB usando GetPoolForTenant()
```

### Ventajas de Esta Estrategia

✅ **Single Sign-On**: Un usuario accede a múltiples tenants con un solo login  
✅ **Seguridad centralizada**: Contraseñas y sesiones en una sola DB auditada  
✅ **No duplicación**: `jose@example.com` existe UNA vez en Main DB  
✅ **Flexibilidad**: Usuario puede ser `owner` en tenant A y `viewer` en tenant B  
✅ **Compliance**: Más fácil cumplir GDPR/SOC2 con usuarios centralizados  

### ¿Qué datos van en cada DB?

| Tipo de Dato | Main DB | Tenant DB |
|--------------|---------|-----------|
| Usuarios (`auth.users`) | ✅ Sí | ❌ No |
| Sesiones (`auth.user_sessions`) | ✅ Sí | ❌ No |
| Tenants (`tenancy.tenants`) | ✅ Sí | ❌ No |
| Membresías (`tenancy.tenant_users`) | ✅ Sí | ❌ No |
| Facturas (`invoices.*`) | ❌ No | ✅ Sí |
| Productos (`products.*`) | ❌ No | ✅ Sí |
| Inventario (`inventory.*`) | ❌ No | ✅ Sí |

## 📚 Arquitectura

### Modelos de Tenancy Soportados

#### 1. Database per Tenant (Recomendado)
Cada tenant tiene su propia base de datos:
```json
{
  "slug": "acme-corp",
  "name": "Acme Corporation",
  "database_url": "postgres://user:pass@host:5432/acme_db",
  "schema_name": "public"
}
```

#### 2. Schema per Tenant (Alternativa)
Múltiples tenants en la misma base de datos con schemas separados:
```json
{
  "slug": "acme-corp",
  "name": "Acme Corporation",
  "database_url": null,
  "schema_name": "acme_schema"
}
```

### Roles de Usuario en Tenants

- **owner**: Control total del tenant
- **admin**: Gestión de usuarios y configuración
- **member**: Acceso a recursos del tenant
- **viewer**: Solo lectura

## 🔌 Endpoints API

### Gestión de Tenants

#### Crear Tenant (System Admin)
```http
POST /api/v1/tenants
Authorization: Bearer {token}
Content-Type: application/json

{
  "slug": "acme-corp",
  "name": "Acme Corporation",
  "database_url": "postgres://...",  // opcional
  "schema_name": "acme_schema"        // opcional
}
```

#### Listar Tenants del Usuario
```http
GET /api/v1/tenants/my-tenants
Authorization: Bearer {token}
```

#### Actualizar Tenant (Owner/Admin)
```http
PATCH /api/v1/tenants/{tenant_slug}
Authorization: Bearer {token}
Content-Type: application/json

{
  "name": "Acme Corporation Inc."
}
```

### Gestión de Usuarios del Tenant

#### Agregar Usuario a Tenant (Owner/Admin)
```http
POST /api/v1/tenants/{tenant_slug}/users
Authorization: Bearer {token}
Content-Type: application/json

{
  "user_id": "uuid-del-usuario",
  "role": "member"
}
```

#### Remover Usuario de Tenant (Owner/Admin)
```http
DELETE /api/v1/tenants/{tenant_slug}/users/{user_id}
Authorization: Bearer {token}
```

## 🔒 Seguridad

### Middleware de Tenancy

El middleware `TenantMiddleware` se aplica automáticamente a rutas tenant-scoped:

1. ✅ Verifica que multitenancy esté habilitado
2. ✅ Extrae `tenant_slug` del URL
3. ✅ Obtiene `user_id` del contexto (AuthMiddleware)
4. ✅ Llama a `sp_verify_user_tenant_access` para validar
5. ✅ Verifica que el tenant esté activo y no suspendido
6. ✅ Inyecta información del tenant en el contexto

### Middleware de Roles

```go
// Solo owner y admin pueden actualizar
adminRoutes.Use(tenantMiddleware.RequireTenantRole("owner", "admin"))
```

### Datos Sensibles

⚠️ **IMPORTANTE**: El `database_url` **NUNCA** se expone en las respuestas de API. Solo se usa internamente para crear connection pools.

## 🎯 Flujo de Uso

### 1. Usuario se autentica
```bash
POST /api/v1/auth/login
```

### 2. Usuario obtiene sus tenants
```bash
GET /api/v1/tenants/my-tenants
```

Respuesta:
```json
[
  {
    "tenant_id": "uuid",
    "slug": "acme-corp",
    "name": "Acme Corporation",
    "is_active": true,
    "user_role": "admin",
    "joined_at": "2024-11-29T10:00:00Z"
  }
]
```

### 3. Usuario accede a recursos del tenant
```bash
GET /api/v1/tenants/acme-corp/...
```

El middleware automáticamente:
- Verifica que el usuario pertenece al tenant
- Conecta al database del tenant (si tiene database_url)
- O usa el schema del tenant (si database_url es NULL)

## 🗄️ Connection Pooling

El `DatabaseService` mantiene pools separados:

- **Primary Pool**: Base de datos principal (tenants, users, etc.)
- **Tenant Pools**: Cache de pools por `database_url`

```go
// Obtener pool del tenant
pool, err := dbService.GetPoolForTenant(ctx, tenant.DatabaseURL)
```

Los pools se crean bajo demanda y se cachean para eficiencia.

## 📝 Traducciones

Todos los errores están traducidos en:
- `modules/tenancy/lang/en.json`
- `modules/tenancy/lang/es.json`

Ejemplos:
```json
{
  "tenant.not-found": "Tenant not found",
  "tenant.user.unauthorized": "You do not have permission to access this tenant",
  "multitenancy.disabled": "Multitenancy is not enabled on this server"
}
```

## 🧪 Testing del Módulo

### 1. Crear un tenant
```bash
curl -X POST http://localhost:8080/api/v1/tenants \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "slug": "test-company",
    "name": "Test Company"
  }'
```

### 2. Agregar usuario al tenant
```bash
curl -X POST http://localhost:8080/api/v1/tenants/test-company/users \
  -H "Authorization: Bearer {token}" \
  -H "Content-Type: application/json" \
  -d '{
    "user_id": "{user-uuid}",
    "role": "owner"
  }'
```

### 3. Listar tenants del usuario
```bash
curl http://localhost:8080/api/v1/tenants/my-tenants \
  -H "Authorization: Bearer {token}"
```

## 🔧 Configuración Avanzada

### Deshabilitar Custom Database URLs

Si solo quieres usar schema-based tenancy:

```env
TENANCY_ALLOW_CUSTOM_DATABASE_URLS=false
```

Los tenants solo podrán usar schemas en la base de datos principal.

### Ajustar Pool Size de Tenants

```env
TENANCY_DATABASE_POOL_SIZE=10
```

Número de conexiones por pool de tenant.

## 📦 Estructura del Módulo

```
modules/tenancy/
├── config/
│   └── config.go              # Configuración del módulo
├── controllers/
│   └── tenant_controller.go   # Endpoints REST
├── errors/
│   └── errors.go              # Constantes de errores
├── interfaces/
│   ├── tenant_repository.go   # Interfaz repository
│   └── tenant_service.go      # Interfaz service
├── lang/
│   ├── en.json                # Traducciones inglés
│   └── es.json                # Traducciones español
├── middleware/
│   └── tenant_middleware.go   # Middleware de acceso
├── migrations/
│   ├── 20251129000001_table_tenants.up.sql
│   ├── 20251129000002_table_tenant_users.up.sql
│   ├── 20251129000003_sp_create_tenant.up.sql
│   └── ...                    # Otros stored procedures
├── models/
│   └── tenant_models.go       # DTOs y entidades
├── repositories/
│   └── tenant_repository.go   # Acceso a datos
├── routes/
│   └── tenant_routes.go       # Registro de rutas
└── services/
    └── tenant_service.go      # Lógica de negocio
```

## ✅ Validaciones

El módulo incluye validaciones a nivel de:

1. **Base de datos** (stored procedures)
   - Slug formato válido (solo lowercase, números, guiones)
   - Nombre requerido
   - Unicidad de slug

2. **Aplicación** (DTOs)
   - Validación con tags `binding:"required"`
   - Normalización con `conform:"trim,lowercase"`

3. **Middleware**
   - Usuario autenticado
   - Usuario pertenece al tenant
   - Tenant activo y no suspendido
   - Rol adecuado para la operación

## 🚨 Troubleshooting

### Multitenancy no disponible
```json
{
  "error": "multitenancy.disabled"
}
```
**Solución**: Configurar `TENANCY_ENABLED=true` y agregar `tenancy` a `ENABLED_MODULES`

### Usuario no autorizado
```json
{
  "error": "tenant.user.unauthorized"
}
```
**Solución**: El usuario debe ser agregado al tenant primero usando `POST /tenants/{slug}/users`

### Tenant no encontrado
```json
{
  "error": "tenant.not-found"
}
```
**Solución**: Verificar que el slug sea correcto y que el tenant exista

## 📖 Referencias

- [Copilot Instructions](../../.github/copilot-instructions.md) - Guías de arquitectura del proyecto
- [Core Module](../core/) - Servicios compartidos
- [Auth Module](../auth/) - Autenticación y JWT
