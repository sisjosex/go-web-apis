# Testing Auth Module

## Setup (Una sola vez)

```powershell
# 1. Crear databases PostgreSQL
psql -U postgres -c "CREATE DATABASE josex;"
psql -U postgres -c "CREATE DATABASE josex_test;"

# 2. Setup test database + migrations
go run cmd/testutil/main.go -setup
```

## Ejecutar Tests

### Unit Tests (Sin BD, Rápido)

```powershell
# Todos los unit tests
go test -v -short ./modules/auth/...

# Solo auth services tests
go test -v -short ./modules/auth/services/
```

### Integration Tests (Con BD Real)

```powershell
# Todos los integration tests
go test -v -tags=integration ./modules/auth/tests/...

# Ejecutar test específico
go test -v -tags=integration -run TestLoginSuccess ./modules/auth/tests/

# Ver output detallado
go test -v -tags=integration ./modules/auth/tests/ -count=1
```

## Limpiar entre Tests

```powershell
# Reset completo: dropea, recrea, migra (recomendado antes de tests)
go run cmd/testutil/main.go -reset

# Solo dropear y recrear (sin migrar)
go run cmd/testutil/main.go -clean

# Solo migrar en DB existente
go run cmd/testutil/main.go -migrate
```

## Full Workflow

```powershell
# 1️⃣ Setup (primera vez)
go run cmd/testutil/main.go -setup

# 2️⃣ Ejecutar unit tests (rápido, sin BD)
go test -v -short ./modules/auth/...

# 3️⃣ Reset DB antes de integration tests
go run cmd/testutil/main.go -reset

# 4️⃣ Ejecutar integration tests (con BD real)
go test -v -tags=integration ./modules/auth/tests/...

# 5️⃣ Ver coverage
go test -coverprofile=coverage.out ./modules/auth/...
go tool cover -html=coverage.out
```

## Casos de Prueba

### Auth API Tests

**Register:**
- ✅ Register con datos válidos
- ❌ Email duplicado
- ❌ Email inválido
- ❌ Password débil
- ❌ Missing fields

**Login:**
- ✅ Login con credenciales válidas
- ✅ Auto-save de tokens a variables
- ❌ Credenciales inválidas
- ❌ User no encontrado
- ❌ Missing email

**Profile:**
- ✅ Get profile (authorized)
- ✅ Update profile
- ❌ Get profile (unauthorized)
- ❌ Update sin autorización

**Password:**
- ✅ Change password con actual correcta
- ✅ New password works después de cambio
- ❌ Change password con actual incorrecta
- ❌ Change sin autorización

**Tokens:**
- ✅ Refresh token válido
- ✅ Token expiration
- ❌ Refresh token inválido
- ❌ Refresh sin autorización

**Logout:**
- ✅ Logout invalidates token
- ❌ Can't use token después de logout

**OTP:**
- ✅ Request OTP email
- ✅ Verify OTP code
- ❌ Invalid channel
- ❌ Missing destination

**Facebook Login:**
- ✅ Login con Facebook ID válido
- ✅ Auto-create user si no existe
- ❌ Missing email

## Variables de Ambiente

### .env.test

```env
DATABASE_URL=postgres://postgres:password@localhost:5432/josex_test
DATABASE_POOL_SIZE=5
SERVER_PORT=8080
GIN_MODE=test
JWT_SECRET_KEY=test-secret-key
OTP_EXPIRY_MINUTES=10
```

## Tips

**Debug:**
```powershell
# Ver queries ejecutadas
go test -v -tags=integration ./modules/auth/tests/ -run TestLoginSuccess 2>&1 | grep -i "query\|error"

# Ejecutar sin parallelization
go test -v -tags=integration -p 1 ./modules/auth/tests/

# Con timeout custom
go test -v -tags=integration -timeout 30s ./modules/auth/tests/
```

**Verificar BD directamente:**
```powershell
# Conectar a test DB
psql -U postgres -d josex_test

# Ver usuarios creados
SELECT id, email, first_name FROM auth.users;

# Ver sesiones
SELECT user_id, created_at FROM auth.user_sessions;

# Ver OTP requests
SELECT destination, otp_channel FROM auth.otp_requests;
```

## Troubleshooting

**Error: "database josex_test does not exist"**
```powershell
psql -U postgres -c "CREATE DATABASE josex_test;"
go run cmd/testutil/main.go -migrate
```

**Error: "connection refused"**
- Verificar que PostgreSQL está corriendo
- Verificar puerto 5432 (dev) y 5433 (test)
- Verificar DATABASE_URL en .env.test

**Error: "migration failed"**
```powershell
go run cmd/testutil/main.go -setup  # Resetea y migra
```

**Tests fallan intermitentemente**
```powershell
# Reset completo antes de tests
go run cmd/testutil/main.go -reset
go test -v -tags=integration -count 1 ./modules/auth/tests/
```
