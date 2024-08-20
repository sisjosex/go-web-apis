# Paso 1: Utilizar una imagen base de Go
FROM golang:1.23-alpine AS builder

# Paso 2: Establecer el directorio de trabajo dentro del contenedor
WORKDIR /app

# Paso 3: Copiar los archivos go.mod y go.sum y descargar las dependencias
COPY go.mod go.sum ./
RUN go mod tidy

# Paso 4: Copiar el código fuente de la aplicación
COPY . .

# Paso 5: Construir la aplicación
RUN go build -o main .

# Paso 6: Configurar el contenedor de ejecución
FROM alpine:3.20

# Establecer el directorio de trabajo en el contenedor final
WORKDIR /app

# Copiar el binario y los archivos necesarios desde el contenedor de construcción
COPY --from=builder /app/main .
COPY --from=builder /app/migrations ./migrations
# COPY .env .env

# Paso 7: Ejecutar la aplicación
CMD ["./main"]
