# ================================
# Etapa de construcción (Build Stage)
# ================================
FROM golang:1.23-alpine AS builder

# Instalar dependencias necesarias para la compilación
RUN apk add --no-cache git ca-certificates tzdata

# Configurar directorio de trabajo
WORKDIR /app

# Copiar archivos de dependencias primero (para optimizar cache de Docker)
COPY go.mod go.sum ./

# Descargar dependencias (se cachea si go.mod no cambia)
RUN go mod download

# Copiar todo el código fuente
COPY . .

# Compilar la aplicación
RUN CGO_ENABLED=0 GOOS=linux go build -a -installsuffix cgo -o book-scraper ./cmd/server

# ================================
# Etapa de producción (Production Stage)
# ================================
FROM alpine:latest

# Instalar certificados CA y zona horaria
RUN apk --no-cache add ca-certificates tzdata

# Crear usuario no-root para seguridad
RUN addgroup -g 1001 -S scraper && \
    adduser -S scraper -u 1001 -G scraper

# Configurar directorio de trabajo
WORKDIR /home/scraper

# Copiar el binario compilado desde la etapa de construcción
COPY --from=builder /app/book-scraper .

# Copiar archivo de configuración (opcional, las variables pueden venir del docker-compose)
COPY --from=builder /app/.env .

# Cambiar permisos del binario
RUN chmod +x book-scraper

# Cambiar al usuario no-root
USER scraper

# Exponer puerto del servicio
EXPOSE 8080

# Health check para Docker
HEALTHCHECK --interval=30s --timeout=3s --start-period=5s --retries=3 \
  CMD wget --no-verbose --tries=1 --spider http://localhost:8080/health || exit 1

# Comando por defecto para ejecutar la aplicación
CMD ["./book-scraper"]