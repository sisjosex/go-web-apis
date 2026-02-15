# OTP Multi-Canal Implementation Summary

## ✅ Completado

Se ha implementado un sistema completo de OTP (One-Time Password) multi-canal siguiendo el patrón Strategy + Dependency Injection del proyecto.

### Componentes Implementados

#### 1. **Migración de Base de Datos** (`20260116170950_table_otp_requests`)
- Tabla `auth.otp_requests` - Agnóstica del canal (WhatsApp, SMS, Email)
- Campos: `destination`, `otp_channel`, `otp_code`, `user_id`, `is_verified`, `attempts`, `expires_at`
- Índices optimizados para búsquedas rápidas
- **Stored Procedures:**
  - `sp_request_otp()` - Genera y almacena código OTP
  - `sp_verify_otp()` - Valida código y crea sesión si usuario existe

#### 2. **Interfaz OTP Provider** (`otp_provider.go`)
```go
type OtpProvider interface {
    SendOtp(ctx context.Context, destination string, otpCode string) (time.Time, error)
    GetChannelName() string
    IsEnabled() bool
}
```

#### 3. **Proveedores Implementados**
- **WhatsApp** (`whatsapp_provider.go`) - Twilio WhatsApp API (mock + TODO)
- **SMS** (`sms_provider.go`) - Twilio SMS API (mock + TODO)
- **Email** (`email_provider.go`) - ✅ **COMPLETADO** - Usa `EmailService` del módulo core

**Email Provider Implementado:**
- Integrado con `coreServices.EmailService` (SMTP automático)
- HTML formateado con estilos inline
- Usa configuración `AuthConfig.SMTP*` (SMTPHost, SMTPPort, SMTPUser, SMTPPass, SMTPFrom)
- En producción, solo necesitas configurar credenciales SMTP

#### 4. **DTOs de OTP** (`auth_dtos.go`)
- `RequestOtpRequestDto` - Request para solicitar OTP
- `RequestOtpDto` - Interno con device info
- `RequestOtpResponse` - Respuesta después de solicitar
- `VerifyOtpRequestDto` - Request para verificar OTP
- `VerifyOtpDto` - Interno con device info
- `VerifyOtpResponse` - Respuesta con session info
- `OtpRequest` - Record de DB

#### 5. **Errores de OTP** (`auth/errors/errors.go`)
```
OtpRequestFailed        - Error al solicitar OTP
OtpGenerated            - OTP enviado exitosamente
OtpVerifyFailed         - Error al verificar
OtpInvalid              - Código OTP inválido
OtpExpired              - OTP expirado
OtpNotFound             - No se encontró OTP válido
OtpMaxAttempts          - Excedido máximo de intentos
OtpChannelInvalid       - Canal no válido
OtpChannelDisabled      - Canal no está habilitado
OtpCodeInvalid          - Formato de código inválido
```

#### 6. **Traducciones** (en.json, es.json)
Todos los mensajes de error con traducciones en inglés y español.

#### 7. **Configuración** (`auth/config/config.go`)
```go
OTPExpiryMinutes       = 10 minutos (configurable)
OTPLength              = 6 dígitos
OTPMaxAttempts         = 5 intentos
OTPEnabledChannels     = ["whatsapp", "sms", "email"]
OTPDefaultChannel      = "whatsapp"
TwilioAccountSID       = Env var
TwilioAuthToken        = Env var
TwilioSmsNumber        = Env var
TwilioWhatsAppNumber   = Env var
```

#### 8. **Servicio OTP** (`otp_service.go`)
- `RequestOtp()` - Solicita OTP:
  1. Valida canal habilitado
  2. Genera código 6-dígitos
  3. Envía vía provider
  4. Almacena en DB
  5. Retorna info con destination enmascarado
- `VerifyOtp()` - Verifica OTP:
  1. Valida canal existe
  2. Consulta DB
  3. Retorna SessionUser si existe usuario, o confirmación de OTP si no

#### 9. **Repositorio OTP** (`otp_repository.go`)
- `RequestOtp()` - Llama `sp_request_otp`
- `VerifyOtp()` - Llama `sp_verify_otp`

#### 10. **Controlador OTP** (`otp_controller.go`)
Tres endpoints para solicitar y uno para verificar:
- `POST /auth/otp/whatsapp/request` - Solicita OTP vía WhatsApp
- `POST /auth/otp/sms/request` - Solicita OTP vía SMS
- `POST /auth/otp/email/request` - Solicita OTP vía Email
- `POST /auth/otp/verify` - Verifica OTP (todos los canales)

#### 11. **Rutas OTP** (`otp_routes.go`, `auth_routes.go`)
- Setup automático de proveedores
- Wiring de DI en `routes.go`
- Documentación Swagger en controladores

#### 12. **Validadores** (`core/validators/global.go`)
- `otp-channel` - Valida canal esté en lista permitida

### Variables de Entorno Requeridas

```env
# OTP Configuration
OTP_EXPIRY_MINUTES=10
OTP_LENGTH=6
OTP_MAX_ATTEMPTS=5
OTP_ENABLED_CHANNELS=whatsapp,sms,email
OTP_DEFAULT_CHANNEL=whatsapp

# Twilio Configuration (para SMS y WhatsApp)
TWILIO_ACCOUNT_SID=your_account_sid
TWILIO_AUTH_TOKEN=your_auth_token
TWILIO_SMS_NUMBER=+1234567890
TWILIO_WHATSAPP_NUMBER=+1234567890

# SMTP (para Email OTP)
SMTP_HOST=mail.smtp2go.com
SMTP_PORT=2525
SMTP_USER=your_email@example.com
SMTP_PASS=your_password
SMTP_FROM=noreply@example.com
```

### Flujos Implementados

#### **Solicitar OTP:**
```
POST /auth/otp/whatsapp/request
{
  "destination": "+1234567890",
  "channel": "whatsapp",
  "device_id": "uuid-here"
}

Response:
{
  "otp_id": "uuid",
  "destination": "+1234****90",
  "channel": "whatsapp",
  "expires_at": "2026-01-16T17:20:00Z",
  "message": "OTP sent to +1234****90 via whatsapp"
}
```

#### **Verificar OTP:**
```
POST /auth/otp/verify
{
  "destination": "+1234567890",
  "otp_code": "123456",
  "channel": "whatsapp",
  "device_id": "uuid-here"
}

Response (usuario existe):
{
  "session_id": "uuid",
  "user_id": "uuid",
  "user_exists": true,
  "system_role": "user",
  "subscription_plan": "free"
}

Response (usuario NO existe):
{
  "session_id": "uuid",
  "user_id": null,
  "user_exists": false
}
```

### Patrón de Diseño

**Strategy Pattern:**
- Cada canal (WhatsApp, SMS, Email) implementa `OtpProvider`
- Intercambiables sin cambiar lógica central
- Fácil agregar nuevos canales

**Dependency Injection:**
- Proveedores inyectados en `OtpService`
- Registry de proveedores en `routes.go`
- Testeable con mocks

### Próximos Pasos

1. **Integración Twilio:**
   - Descomentar código en `whatsapp_provider.go` y `sms_provider.go`
   - Instalar SDK: `go get github.com/twilio/twilio-go`
   - Usar credenciales en `AuthConfig`: TwilioAccountSID, TwilioAuthToken, TwilioSmsNumber, TwilioWhatsAppNumber

2. ✅ **Integración Email:**
   - **COMPLETADA** - Usa el `EmailService` del módulo core
   - `SendPlainEmail()` con HTML formateado
   - Automáticamente usa configuración SMTP de `AuthConfig`

3. **JWT Tokens en Login OTP:**
   - En `otp_controller.go`, línea ~190, generar JWT usando `JWTService`
   - Retornar `access_token` y `refresh_token` cuando usuario existe
   - Crear endpoint `POST /auth/register/otp` para crear usuario después de verificar OTP
   - Usar destination (phone/email) como dato inicial

5. **Rate Limiting:**
   - Agregar `tollbooth` en `/auth/otp/*` (máximo 3 requests/minuto por IP)
   - Validar en PostgreSQL: solo 1 OTP cada 60 segundos por destination/channel

6. **Cleanup de OTPs Expirados:**
   - Crear job que delete OTPs expirados > 24 horas

### Archivos Modificados

- `modules/auth/migrations/20260116170950_table_otp_requests.{up,down}.sql` - Migración
- `modules/auth/services/otp/{otp_provider.go, whatsapp_provider.go, sms_provider.go, email_provider.go}` - Proveedores
- `modules/auth/services/otp_service.go` - Servicio
- `modules/auth/repositories/otp_repository.go` - Repositorio
- `modules/auth/controllers/otp_controller.go` - Controlador
- `modules/auth/routes/{otp_routes.go, auth_routes.go}` - Rutas
- `modules/auth/interfaces/otp_interfaces.go` - Interfaces
- `modules/auth/models/auth_dtos.go` - DTOs
- `modules/auth/errors/errors.go` - Errores
- `modules/auth/lang/{en.json, es.json}` - Traducciones
- `modules/auth/config/config.go` - Configuración
- `modules/core/validators/global.go` - Validadores
- `routes/routes.go` - Main routes wiring

### Testing Local

Para probar localmente SIN Twilio:

1. Las implementaciones mock logean a consola
2. Ver logs: `📱 [MOCK] WhatsApp OTP sent to +1234567890: 123456`
3. Usar ese código en `/auth/otp/verify`

### Monitoreo

En producción, monitorear:
- OTP rate (cuántos se solicitan/verifican por minuto)
- OTP success rate (cuántos se verifican exitosamente)
- Canal usage (cuál canal es más usado)
- Errores por destino (teléfono/email con problema)
