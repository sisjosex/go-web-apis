# Guía de Refactorización Modular - Go Web API

## 📁 Nueva Estructura de Módulos

```
modules/
├── auth/                           # Módulo de Autenticación
│   ├── controllers/
│   │   ├── auth_controller.go     # Login, Register, Password Reset
│   │   └── session_controller.go  # Session Management
│   ├── services/
│   │   ├── auth_service.go        # Lógica de autenticación
│   │   └── jwt_service.go         # JWT token generation
│   ├── repositories/
│   │   └── auth_repository.go     # Auth stored procedures
│   ├── models/
│   │   └── auth_dtos.go           # Auth-specific DTOs
│   └── routes/
│       └── auth_routes.go         # Auth routes setup
│
├── users/                          # Módulo de Gestión de Usuarios
│   ├── controllers/
│   │   └── user_controller.go     # CRUD operations
│   ├── services/
│   │   └── user_service.go        # User business logic
│   ├── repositories/
│   │   └── user_repository.go     # User stored procedures
│   ├── models/
│   │   └── user_dtos.go           # User-specific DTOs
│   └── routes/
│       └── user_routes.go         # User routes setup
│
└── shared/                         # Código Compartido
    ├── models/
    │   ├── user.go                # User entity
    │   ├── datetime.go            # Custom DateTime type
    │   ├── date_only.go           # Custom DateOnly type
    │   ├── session_user.go        # Session data
    │   └── user_session.go        # Active session
    ├── repositories/
    │   └── base_repository.go     # DB pool & common methods
    └── services/
        └── validator_service.go   # Shared validations
```

## 🔄 Mapeo de Archivos

### Archivos Creados (✅ Completado)

| Archivo Original | Nuevo Archivo | Estatus |
|-----------------|---------------|---------|
| `models/user.go` | `modules/shared/models/user.go` | ✅ |
| `models/datetime.go` | `modules/shared/models/datetime.go` | ✅ |
| `models/date_only.go` | `modules/shared/models/date_only.go` | ✅ |
| `models/session_user.go` | `modules/shared/models/session_user.go` | ✅ |
| `models/user_session.go` | `modules/shared/models/user_session.go` | ✅ |
| `models/login_user_dto.go` + otros | `modules/auth/models/auth_dtos.go` | ✅ |
| `models/create_user_dto.go` + otros | `modules/users/models/user_dtos.go` | ✅ |
| - | `modules/shared/repositories/base_repository.go` | ✅ |

### Archivos Pendientes (⏳ Por Completar)

| Archivo Original | Acción | Nuevo Archivo |
|-----------------|--------|---------------|
| `repositories/user_repository.go` | **Dividir** | ⏳ `modules/auth/repositories/auth_repository.go`<br>⏳ `modules/users/repositories/user_repository.go` |
| `services/user_service.go` | **Dividir** | ⏳ `modules/auth/services/auth_service.go`<br>⏳ `modules/users/services/user_service.go` |
| `services/jwt_service.go` | **Mover** | ⏳ `modules/auth/services/jwt_service.go` |
| `controllers/auth_controller.go` | **Mover** | ⏳ `modules/auth/controllers/auth_controller.go` |
| `controllers/session_controller.go` | **Mover** | ⏳ `modules/auth/controllers/session_controller.go` |
| `controllers/user_controller.go` | **Mover** | ⏳ `modules/users/controllers/user_controller.go` |
| `routes/routes.go` | **Dividir** | ⏳ `modules/auth/routes/auth_routes.go`<br>⏳ `modules/users/routes/user_routes.go`<br>⏳ `routes/routes.go` (orchestrator) |

## 📝 División de Métodos de Repositorio

### Auth Repository
**Métodos de autenticación y sesiones:**
```go
- LoginUser(LoginUserDto) (*SessionUser, error)
- LoginExternal(LoginExternalDto) (*SessionUser, error)
- LogoutUser(LogoutSessionDto) (*bool, error)
- ValidateSession(userID, sessionID uuid.UUID) error
- GetUserSessions(userID uuid.UUID) ([]UserSession, error)
- LogoutSession(userID, sessionID uuid.UUID) error
- LogoutAllSessions(userID uuid.UUID, currentSessionID *uuid.UUID) (int, error)
- GetProfile(GetProfileDto) (*User, error)
- UpdateProfile(UpdateProfileDto) (*User, error)
- GenerateEmailVerificationToken(VerifyEmailRequest, pgx.Tx) (*VerifyEmailToken, error)
- ConfirmEmailAddress(VerifyEmailToken) (*bool, error)
- ChangePassword(ChangePasswordDto) (*bool, error)
- GeneratePasswordResetToken(PasswordResetRequestDto, pgx.Tx) (*PasswordResetTokenRequestDto, error)
- ResetPasswordWithToken(PasswordResetWithTokenDto) (*bool, error)
```

### User Repository
**Métodos CRUD administrativos:**
```go
- InsertUser(CreateUserDto) (*User, error)
- UpdateUser(UpdateUserDto) (*User, error)
- ListUsers(UserListQuery) (*UserListResponse, error)
- GetUserById(userID uuid.UUID) (*User, error)
- SoftDeleteUser(userID uuid.UUID, reason string) error
```

## 🔧 Pasos de Implementación

### Fase 1: Modelos ✅ COMPLETADO
- [x] Crear `modules/shared/models/` con modelos compartidos
- [x] Crear `modules/auth/models/auth_dtos.go`
- [x] Crear `modules/users/models/user_dtos.go`

### Fase 2: Repositorios (⏳ EN PROGRESO)
- [ ] Crear `modules/shared/repositories/base_repository.go`
- [ ] Dividir `repositories/user_repository.go`:
  - [ ] Crear `modules/auth/repositories/auth_repository.go`
  - [ ] Crear `modules/users/repositories/user_repository.go`

### Fase 3: Servicios
- [ ] Mover `services/jwt_service.go` → `modules/auth/services/jwt_service.go`
- [ ] Dividir `services/user_service.go`:
  - [ ] Crear `modules/auth/services/auth_service.go`
  - [ ] Crear `modules/users/services/user_service.go`

### Fase 4: Controladores
- [ ] Mover `controllers/auth_controller.go` → `modules/auth/controllers/`
- [ ] Mover `controllers/session_controller.go` → `modules/auth/controllers/`
- [ ] Mover `controllers/user_controller.go` → `modules/users/controllers/`

### Fase 5: Rutas
- [ ] Crear `modules/auth/routes/auth_routes.go`
- [ ] Crear `modules/users/routes/user_routes.go`
- [ ] Actualizar `routes/routes.go` para orquestar módulos

### Fase 6: Actualizar Imports
- [ ] Actualizar imports en `interfaces/`
- [ ] Actualizar imports en `common/`
- [ ] Actualizar imports en `utils/`
- [ ] Actualizar imports en `middleware/`
- [ ] Actualizar imports en `main.go`

### Fase 7: Pruebas
- [ ] Ejecutar `go mod tidy`
- [ ] Ejecutar `go build`
- [ ] Probar endpoints de auth
- [ ] Probar endpoints de users
- [ ] Regenerar Swagger

## 🎯 Beneficios de la Nueva Estructura

1. **Separación de Responsabilidades**
   - Auth: Autenticación, autorización, sesiones
   - Users: CRUD administrativo de usuarios
   - Shared: Modelos y utilidades comunes

2. **Reutilización de Código**
   - Base Repository: Pool de DB compartido
   - Shared Models: Un solo lugar para User, DateTime, etc.
   - No duplicación de código

3. **Escalabilidad**
   - Fácil agregar nuevos módulos (Products, Orders, etc.)
   - Cada módulo es independiente
   - Preparado para microservicios futuros

4. **Mantenibilidad**
   - Código organizado por dominio
   - Fácil encontrar archivos
   - Interfaces claras entre módulos

## 📖 Convenciones de Imports

```go
// Modelos compartidos
import sharedModels "josex/web/modules/shared/models"

// Repositorio compartido
import sharedRepos "josex/web/modules/shared/repositories"

// Modelos de Auth
import authModels "josex/web/modules/auth/models"

// Modelos de Users
import userModels "josex/web/modules/users/models"

// Servicios de Auth
import authServices "josex/web/modules/auth/services"

// Uso en código
user := sharedModels.User{}
loginDto := authModels.LoginUserRequestDto{}
baseRepo := sharedRepos.NewBaseRepository(dbService)
```

## 🚀 Comando Rápido de Verificación

```powershell
# Compilar y verificar
go mod tidy
go build -o bin/server.exe .

# Regenerar Swagger
swag init

# Ejecutar
go run .
```
