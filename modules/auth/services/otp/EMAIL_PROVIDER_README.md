# Email OTP Provider Implementation

## ✅ Completado

El `EmailProvider` ha sido integrado con el `EmailService` del módulo core.

## Componentes

### EmailProvider Structure
```go
type EmailProvider struct {
    config       *config.AuthConfig      // Config con SMTP settings
    emailService coreServices.EmailService  // Injected from core module
    isEnabled    bool                    // Is SMTP configured?
}
```

### Métodos

#### `NewEmailProvider(authConfig)`
- Inicializa el provider
- Inyecta automáticamente `EmailService` del core
- Valida que SMTP esté configurado (SMTPHost y SMTPFrom no vacíos)

#### `SendOtp(ctx, destination, otpCode)`
- Construye HTML formateado con estilos inline
- Llama `emailService.SendPlainEmail()`
- Automáticamente usa credenciales SMTP desde `AuthConfig`
- Retorna expiration time (10 minutos)

## Configuración Requerida

```env
# En .env.platform y .env.tenant

# SMTP Configuration (necesario para OTP por Email)
SMTP_HOST=mail.smtp2go.com
SMTP_PORT=2525
SMTP_USER=your_email@example.com
SMTP_PASS=your_password
SMTP_FROM=noreply@example.com
```

## Flujo

1. Cliente solicita OTP: `POST /auth/otp/email/request`
2. OtpController valida destino y canal
3. OtpService selecciona EmailProvider
4. EmailProvider construye HTML y llama `emailService.SendPlainEmail()`
5. EmailService usa net/smtp para enviar
6. OtpRepository guarda en `auth.otp_requests`
7. Response al cliente con destination enmascarado

## Email Template

```html
<html>
    <body style="font-family: Arial, sans-serif;">
        <div style="padding: 20px; background-color: #f5f5f5; border-radius: 5px;">
            <h2 style="color: #333;">Your One-Time Password</h2>
            <p style="color: #666; font-size: 16px;">
                Your verification code is:
            </p>
            <div style="background-color: #fff; padding: 15px; border-radius: 5px; margin: 20px 0;">
                <code style="font-size: 32px; font-weight: bold; color: #007bff; letter-spacing: 5px;">
                    [OTP_CODE]
                </code>
            </div>
            <p style="color: #999; font-size: 14px;">
                This code will expire in 10 minutes.
            </p>
        </div>
    </body>
</html>
```

## Proveedores Alternativos Futuros

El patrón Strategy permite agregar nuevos providers sin cambiar la lógica principal:

- **Telegram** - Usar Telegram Bot API
- **Slack** - Usar Slack Webhook
- **Push Notification** - Usar Firebase Cloud Messaging
- **SMS (Vonage)** - Alternativa a Twilio

Simplemente crear una struct que implemente `OtpProvider` y registrarla en `routes/otp_routes.go`.

## Testing Local

Para probar sin SMTP configurado:
1. Los logs mostrarán: `📧 OTP email sent successfully to user@example.com`
2. En producción, el email se enviará automáticamente
3. El código imprime si hay error SMTP

## Fuente

- **Interface**: `modules/auth/services/otp/otp_provider.go`
- **Implementación**: `modules/auth/services/otp/email_provider.go`
- **Integración**: `modules/core/services/email_service.go`
