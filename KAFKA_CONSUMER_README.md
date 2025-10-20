# Kafka Consumer para Web Scraping - Guía de Implementación

## Cambios Implementados

### 1. Evento de Kafka Mejorado
- **Antes**: Se enviaba solo el título del libro
- **Ahora**: Se envía título + menor precio encontrado usando la estructura `KafkaEvent`

### 2. Consumer de Kafka Implementado
- **Función**: Escucha el topic `recommendation-scrapping`
- **Acción**: Guarda automáticamente libros únicos con su menor precio
- **Ejecución**: Se ejecuta en background como goroutine

### 3. Nueva Tabla en Base de Datos
- **Tabla**: `unique_books`
- **Propósito**: Almacenar cada libro una sola vez con su precio más bajo
- **Lógica**: Si llega un precio menor, actualiza el registro

## Configuración Requerida

### 1. Crear la Tabla de Libros Únicos
```sql
-- Ejecutar este script en MySQL
USE book_prices;

CREATE TABLE IF NOT EXISTS unique_books (
    id INT AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    min_price DECIMAL(10,2) NOT NULL,
    source VARCHAR(100) NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    INDEX idx_title (title),
    INDEX idx_min_price (min_price),
    INDEX idx_updated_at (updated_at),
    
    UNIQUE KEY unique_title (title)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
```

### 2. Variables de Entorno
Asegúrate de que tu `.env` contenga:
```env
# Kafka Configuration
KAFKA_BROKER=kafka:9092
KAFKA_TOPIC=recommendation-scrapping

# Database Configuration  
DB_HOST=localhost
DB_PORT=3312
DB_USER=root
DB_PASSWORD=password
DB_NAME=book_prices
```

## Nuevos Endpoints

### GET /unique-books
Obtiene todos los libros únicos con sus menores precios
```json
{
  "unique_books": [
    {
      "id": 1,
      "title": "Don Quijote",
      "min_price": 45000,
      "source": "Buscalibre",
      "updated_at": "2025-10-19T10:30:00Z"
    }
  ],
  "count": 1,
  "timestamp": "2025-10-19T10:30:00Z"
}
```

## Flujo de Funcionamiento

### 1. Web Scraping (GET /scrape/:book)
1. Hace scraping del libro en las fuentes configuradas
2. Guarda todos los precios en `book_prices`
3. Calcula el menor precio
4. **ENVÍA EVENTO A KAFKA** con título y menor precio

### 2. Consumer de Kafka (Background)
1. Escucha continuamente el topic `recommendation-scrapping`
2. Recibe eventos con estructura:
   ```json
   {
     "book_title": "Don Quijote",
     "min_price": 45000,
     "source": "web_scraper_service",
     "action": "scraped"
   }
   ```
3. **GUARDA/ACTUALIZA** en tabla `unique_books`:
   - Si el libro no existe → lo inserta
   - Si existe y el nuevo precio es menor → lo actualiza
   - Si existe y el nuevo precio es mayor → no hace nada

## Cómo Probar

### 1. Iniciar el Servicio
```bash
cd Back-Web_Scraping
go run cmd/server/main.go
```

### 2. Hacer Scraping de un Libro
```bash
curl "http://localhost:8080/scrape/Don%20Quijote"
```

### 3. Verificar Libros Únicos
```bash
curl "http://localhost:8080/unique-books"
```

### 4. Verificar Logs
El consumer estará activo y verás logs como:
```
🎧 Iniciando consumer de Kafka para topic: recommendation-scrapping
✅ Consumer suscrito exitosamente al topic: recommendation-scrapping
📥 Mensaje recibido de Kafka - Topic: recommendation-scrapping
📚 Procesando evento - Libro: 'Don Quijote', Precio: $45000
✅ Libro único guardado/actualizado exitosamente: 'Don Quijote' con precio $45000
```

## Ventajas de esta Implementación

1. **Separación de Responsabilidades**: El scraping y el almacenamiento están desacoplados
2. **Escalabilidad**: Múltiples consumers pueden procesar eventos
3. **Tolerancia a Fallos**: Si el consumer falla, los eventos quedan en Kafka
4. **Datos Únicos**: Cada libro se almacena una sola vez con su mejor precio
5. **Tiempo Real**: Los eventos se procesan inmediatamente

## Estructura de Archivos Modificados/Creados

```
Back-Web_Scraping/
├── internal/
│   ├── models/book.go                     # ✅ Agregado KafkaEvent y UniqueBook
│   ├── handlers/handlers.go               # ✅ Modificado ScrapeBook + nuevo endpoint
│   ├── kafka/
│   │   ├── producer.go                    # ✅ Ya existía
│   │   └── consumer.go                    # 🆕 Nuevo consumer
│   └── db/mysql.go                        # ✅ Agregados métodos para unique_books
├── cmd/server/main.go                     # ✅ Integrado consumer en goroutine
└── scripts/create_unique_books_table.sql  # 🆕 Script SQL para crear tabla
```