# ================================
# Etapa de construcción (Build Stage)
# ================================
FROM golang:1.23-bullseye AS builder

# Instalar dependencias necesarias para compilar con CGO y Kafka
RUN apt-get update && apt-get install -y \
    build-essential \
    pkg-config \
    librdkafka-dev \
    ca-certificates \
    git \
    tzdata && \
    rm -rf /var/lib/apt/lists/*

# Configurar directorio de trabajo
WORKDIR /app

# Copiar archivos de dependencias primero (para aprovechar cache)
COPY go.mod go.sum ./

# Descargar dependencias Go
RUN go mod download

# Copiar el código fuente completo
COPY . .

# Compilar la aplicación con soporte CGO (requerido por confluent-kafka-go)
RUN CGO_ENABLED=1 GOOS=linux go build -a -o book-scraper ./cmd/server


# ================================
# Etapa de producción (Production Stage)
# ================================
FROM debian:bullseye-slim

# Instalar librerías necesarias para ejecutar el binario compilado
RUN apt-get update && apt-get install -y \
    librdkafka1 \
    ca-certificates \
    tzdata \
    curl \
    wget && \
    rm -rf /var/lib/apt/lists/*

# Crear usuario no-root por seguridad
RUN groupadd -g 1001 scraper && \
    useradd -r -u 1001 -g scraper scraper

# Configurar directorio de trabajo
WORKDIR /home/scraper

# Copiar binario desde el builder
COPY --from=builder /app/book-scraper .

# Copiar archivo .env (opcional, puedes eliminar esta línea si usas solo variables de entorno)
COPY --from=builder /app/.env .

# Cambiar permisos del ejecutable
RUN chmod +x book-scraper

# Cambiar al usuario no-root
USER scraper

# Exponer puerto del servicio
EXPOSE 8080

# Health check para Docker
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Comando por defecto para ejecutar la app
CMD ["./book-scraper"]
