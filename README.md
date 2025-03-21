# Optimized Backend with Go, Gin, and PostgreSQL

Este backend está optimizado para manejar múltiples conexiones a PostgreSQL utilizando un pool de conexiones. Se basa en Gin, un framework web rápido y ligero para Go.

## Requisitos

Antes de ejecutar el proyecto, asegúrate de tener instaladas las siguientes dependencias:

- **Golang**: Instala la última versión desde [golang.org](https://golang.org/dl/).
- **PostgreSQL**: Base de datos relacional utilizada en el backend.
- **Docker & Docker Compose** (Opcional): Para ejecutar el entorno en contenedores.

## Tecnologías y Librerías

Este proyecto utiliza las siguientes tecnologías y librerías:

- **Go**: Lenguaje de programación principal.
- **Gin**: Framework web en Go para manejar solicitudes HTTP.
- **PostgreSQL**: Base de datos utilizada para almacenar información.
- **Docker Compose**: Herramienta para definir y gestionar entornos en contenedores.

---

## Cómo ejecutar el proyecto localmente

Sigue estos pasos para ejecutar el backend en tu máquina local:

1. Renombra el archivo de entorno de desarrollo:

   ```sh
   mv .env.sample-dev .env
   ```

2. Descarga las dependencias del proyecto ejecutando:

   ```sh
   go mod tidy
   ```

3. Inicia el servidor ejecutando:

   ```sh
   go run .
   ```

   Esto ejecutará el servidor en el puerto configurado en el código.

---

## Construcción y ejecución con Docker

Para construir y ejecutar el backend con Docker, sigue estos pasos:

### 1. Construir la imagen Docker

```sh
docker build -t go-web:1 .

# Multiplataforma (buildx)
docker buildx create --use
docker buildx build --platform linux/amd64,linux/arm64 -t tu_usuario/tu_imagen:latest .

#subir a dockerhub
docker buildx build --platform linux/amd64,linux/arm64 -t tu_usuario/tu_imagen:latest --push .
```

Esto crea una imagen Docker con el backend en Go.

### 2. Ejecutar el contenedor

```sh
docker run -p 8080:8080 -e DATABASE_URL=postgres://postgres:dbpassword@<server-ip>:5432/web?sslmode=disable go-web:1
```

Reemplaza `<server-ip>` con la IP del servidor donde se ejecuta PostgreSQL. Este comando:

- Asigna el puerto `8080` en el host al puerto `8080` del contenedor.
- Define la variable de entorno `DATABASE_URL` para la conexión a PostgreSQL.
- Ejecuta el backend dentro del contenedor Docker.

---

## Uso de Docker Compose

Si deseas construir y ejecutar el backend junto con PostgreSQL utilizando Docker Compose, sigue estos pasos:

### 1. Renombra el archivo de entorno para Docker:

```sh
mv .env.sample-docker .env.docker
```

### 2. Construcción de la imagen con Docker Compose

```sh
docker compose --env-file .env.docker build
```

Esto crea las imágenes necesarias utilizando la configuración definida en `docker-compose.yml` y las variables de `.env.docker`.

### 3. Iniciar los contenedores

```sh
docker compose --env-file .env.docker up
```

Este comando inicia todos los servicios definidos en `docker-compose.yml`, como el backend y la base de datos.

Si necesitas construir la imagen y ejecutarla en un solo paso, usa:

```sh
docker compose --env-file .env.docker up --build
```

### 4. Para usar una imagen ya existente subida a dockerhub:

```sh
docker compose --env-file .env.docker -f docker-compose.prod.yml up
```

Para dar de baja los contenedores

```sh
docker compose --env-file .env.docker down
```

### 5. Como probar la aplicacion

Abrir swagger en el navegador `http://localhost:8080/swagger/index.html` o el puerto y host que se tenga configurado.

```
http://<HOST>:<PORT>/swagger/index.html
```

---


## Ejemplos de APIs

Aquí tienes ejemplos de cómo consumir las APIs de autenticación utilizando `curl`.

### Registro de usuario

```sh
curl --location 'http://localhost:8080/api/v1/auth/register' \
--header 'Content-Type: application/json' \
--data-raw '{
    "first_name": "Jhon",
    "last_name": "Smith",
    "email": "jhon-smith@mail.com",
    "password": "$Password2025",
    "phone": "11111111",
    "birthday": "1985-01-01",
    "profile_picture_url": "http://fotos.com/profile.png",
    "bio": "some bio",
    "website_url": "http://page.com"
}'
```

### Inicio de sesión

```sh
curl --location 'http://localhost:8080/api/v1/auth/login' \
--header 'User-Agent: Mozilla/5.0 (X11; Ubuntu; Linux x86_64; rv:89.0; Device=Laptop) Gecko/20100101 Firefox/89.0' \
--header 'Content-Type: application/json' \
--data-raw '{
    "email": "jhon-smith@mail.com",
    "password": "$Password2025"
}'

```

Estos ejemplos pueden ejecutarse en una terminal o Postman para interactuar con el backend.

---

## Códigos de error

_(Aquí puedes agregar una lista de códigos de error que pueda devolver el backend y su significado.)_

---
