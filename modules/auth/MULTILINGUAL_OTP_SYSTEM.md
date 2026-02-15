# Multilingual OTP Email System

## ✅ Completado: Sistema OTP Multi-lenguaje

El sistema OTP ahora soporta múltiples idiomas automáticamente usando el mismo template HTML con placeholders traducidos.

## Arquitectura

### Flujo de Lenguaje

```
Request (con ?lang=es o Accept-Language: es)
    ↓
LanguageMiddleware (extrae idioma)
    ↓
OtpController (captura idioma del contexto de Gin)
    ↓
Crea context.Context con valor "lang"
    ↓
OtpService (extrae idioma del context)
    ↓
Provider (recibe idioma como parámetro)
    ↓
EmailProvider (traduce placeholders usando GetTranslation)
    ↓
Template único + datos traducidos
    ↓
EmailService.SendEmail()
```

## Componentes Actualizados

### 1. **Interfaz OtpProvider**
```go
SendOtp(ctx context.Context, destination string, otpCode string, lang string) (time.Time, error)
```
- Ahora recibe `lang` como parámetro

### 2. **Proveedores (WhatsApp, SMS, Email)**
- Todos actualizados para aceptar parámetro `lang`
- Mock logs incluyen el idioma: `(lang=es)`

### 3. **EmailProvider (Implementación Real)**
```go
templateData := map[string]string{
    "OtpCode":         otpCode,
    "Title":           coreUtils.GetTranslation(lang, "otp.email.title"),
    "Greeting":        coreUtils.GetTranslation(lang, "otp.email.greeting"),
    // ... más placeholders
}
```
- Traduce todos los textos dinámicamente
- Un solo template, múltiples idiomas

### 4. **OtpService**
```go
// Extrae idioma del context
lang := "en"
if langValue := ctx.Value("lang"); langValue != nil {
    if langStr, ok := langValue.(string); ok {
        lang = langStr
    }
}

// Pasa al provider
expiresAt, err := provider.SendOtp(ctx, dto.Destination, otpCode, lang)
```

### 5. **OtpController**
```go
// Crea context con idioma desde Gin
ctx := c.Request.Context()
if lang, exists := c.Get("lang"); exists {
    ctx = context.WithValue(ctx, "lang", lang)
}

response, err := uc.otpService.RequestOtp(ctx, otpDto)
```

## Template Único

**Archivo:** `modules/auth/templates/otp-code.html`

```html
<h2>{{.Greeting}}</h2>
<p>{{.Description}}</p>
<div class="otp-code">{{.OtpCode}}</div>
<p>{{.ExpirationText}}</p>
<p>{{.SecurityWarning}}</p>
```

**Placeholders (9 totales):**
- `{{.OtpCode}}` - El código
- `{{.Title}}` - Título HTML
- `{{.Greeting}}` - "Hello," / "Hola,"
- `{{.Description}}` - Descripción del OTP
- `{{.CodeLabel}}` - Etiqueta "Your code is:" / "Tu código es:"
- `{{.ExpirationText}}` - Info de expiración
- `{{.SecurityWarning}}` - Advertencia de seguridad
- `{{.IgnoreText}}` - Texto si no solicitó
- `{{.SignOff}}` - Firma del equipo
- `{{.AutomatedNote}}` - Nota de correo automático

## Traducciones

### Inglés (`modules/auth/lang/en.json`)
```json
{
  "otp.email.subject": "Your OTP Code",
  "otp.email.title": "Your OTP Code",
  "otp.email.greeting": "Hello,",
  "otp.email.description": "You requested a verification code to access your account.",
  "otp.email.code-label": "Your verification code is:",
  "otp.email.expiration": "This code expires in 10 minutes",
  "otp.email.security-warning": "Security: Never share this code with anyone...",
  "otp.email.ignore-text": "If you did not request this code, you can safely ignore this email.",
  "otp.email.sign-off": "Best regards,<br><strong>The Xanthops Team</strong>",
  "otp.email.automated-note": "This is an automated email, please do not reply to this message."
}
```

### Español (`modules/auth/lang/es.json`)
```json
{
  "otp.email.subject": "Tu código OTP",
  "otp.email.title": "Tu código OTP",
  "otp.email.greeting": "Hola,",
  "otp.email.description": "Solicitaste un código de verificación para acceder a tu cuenta.",
  "otp.email.code-label": "Tu código de verificación es:",
  "otp.email.expiration": "Este código expira en 10 minutos",
  "otp.email.security-warning": "Seguridad: Nunca compartas este código con nadie...",
  "otp.email.ignore-text": "Si no solicitaste este código, puedes ignorar este correo de forma segura.",
  "otp.email.sign-off": "Saludos,<br><strong>El equipo de Xanthops</strong>",
  "otp.email.automated-note": "Este es un correo automático, por favor no respondas a este mensaje."
}
```

## Ejemplos de Uso

### Solicitar OTP en Inglés
```bash
curl -X POST http://localhost:8080/api/v1/otp/email/request \
  -H "Accept-Language: en" \
  -H "Content-Type: application/json" \
  -d '{
    "destination": "user@example.com",
    "channel": "email",
    "device_id": "uuid"
  }'
```

### Solicitar OTP en Español (vía query param)
```bash
curl -X POST "http://localhost:8080/api/v1/otp/email/request?lang=es" \
  -H "Content-Type: application/json" \
  -d '{
    "destination": "usuario@ejemplo.com",
    "channel": "email",
    "device_id": "uuid"
  }'
```

### Solicitar OTP en Español (vía header)
```bash
curl -X POST http://localhost:8080/api/v1/otp/email/request \
  -H "Accept-Language: es" \
  -H "Content-Type: application/json" \
  -d '{"destination": "usuario@ejemplo.com", "channel": "email", "device_id": "uuid"}'
```

## Ventajas

✅ **Un solo template** - Mantenimiento simplificado
✅ **Dinámicamente traducido** - Idioma detectado automáticamente
✅ **Reutilizable** - `GetTranslation()` del core
✅ **Extensible** - Agregar nuevos idiomas = agregar claves a JSON
✅ **Performance** - Sin I/O extra, todo en memoria
✅ **Consistente** - Mismo mecanismo que otros módulos

## Agregar Nuevo Idioma

1. Crear `modules/auth/lang/fr.json` con las claves:
```json
{
  "otp.email.subject": "Votre code OTP",
  "otp.email.title": "Votre code OTP",
  // ... resto de claves
}
```

2. Actualizar `.env`:
```env
ENABLED_LANGUAGES=en,es,fr
```

¡Listo! El sistema automáticamente detectará francés en `Accept-Language: fr` o `?lang=fr`

## Testing Local

Logs muestran idioma usado:
```
📧 OTP email sent successfully to user@example.com (lang=es)
💬 [MOCK] SMS OTP sent to +1234567890 (lang=en)
📱 [MOCK] WhatsApp OTP sent to +1234567890 (lang=fr)
```
