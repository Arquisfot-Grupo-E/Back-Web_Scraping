# 🕷️ Back-Web_Scraping

Microservicio de web scraping para la obtención automatizada de precios de libros en tiempo real. Desarrollado en **Go** con **Gin**, **MySQL** y **Apache Kafka** para proporcionar datos actualizados de precios desde múltiples fuentes web.

## 🚀 Características principales

### ✅ **Web Scraping Inteligente**
- Scraping automatizado de: **Buscalibre** 
- Sistema de reintentos y manejo de errores robusto
- User-Agent configurable y respeto por robots.txt
- Timeout configurable para optimizar rendimiento

### ✅ **Sistema de Mensajería con Kafka**
- **Producer**: Envía eventos cuando se encuentra un nuevo precio mínimo
- **Consumer**: Procesa eventos en tiempo real para almacenar libros únicos
- Topic: `recommendation-scrapping` para integración con servicios de recomendaciones
- Procesamiento asíncrono en background con goroutines

### ✅ **Base de Datos MySQL Optimizada**
- Tabla `book_prices`: Almacena todos los precios encontrados con historial
- Tabla `unique_books`: Mantiene el precio mínimo por libro (actualizado automáticamente)
- Índices optimizados para consultas rápidas por título y precio
- Constraints de unicidad para evitar duplicados

### ✅ **API REST Completa**
- Documentación automática en endpoint raíz (`/`)
- Health check para monitoreo (`/health`)
- Endpoints para scraping manual y consultas de datos
- Middleware CORS para integración con frontend
- Logging estructurado para debugging

## 🛠️ Instalación

### 1. Clona el repositorio
```bash
git clone <url-del-repositorio>
cd Back-Web_Scraping
```

### 2. Docker (Recomendado)
```bash
docker-compose up --build
```

El servicio estará disponible en: `http://localhost:8080`

### 3. Instalación Manual (Desarrollo)

#### Prerrequisitos
- **Go 1.23+**
- **MySQL 8.0+**
- **Apache Kafka** (para mensajería)

```bash
# Instalar dependencias
go mod download

# Configurar base de datos
mysql -u root -p < scripts/init.sql
mysql -u root -p < scripts/create_unique_books_table.sql

# Configurar variables de entorno
cp .env.example .env
# Editar .env con tus configuraciones

# Ejecutar el servicio
go run cmd/server/main.go
```

## 📚 API Endpoints

### Endpoints Principales
| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/` | Documentación de la API y estado del servicio |
| GET | `/health` | Health check del servicio |
| GET | `/scrape/:book` | Realizar scraping manual de un libro |
| GET | `/books` | Obtener historial completo de precios |
| GET | `/unique-books` | Obtener libros únicos con precios mínimos |
| POST | `/enqueue` | Encolar libro para scraping asíncrono |

### Ejemplos de Uso

#### Hacer scraping de un libro
```bash
curl "http://localhost:8080/scrape/Don%20Quijote"
```

#### Obtener todos los precios únicos
```bash
curl "http://localhost:8080/unique-books"
```

**Respuesta esperada:**
```json
{
  "unique_books": [
    {
      "id": 1,
      "title": "Don Quijote",
      "min_price": 45000,
      "source": "Buscalibre",
      "updated_at": "2025-10-22T10:30:00Z"
    }
  ],
  "count": 1,
  "timestamp": "2025-10-22T10:30:00Z"
}
```

## 🔧 Configuración

### Variables de Entorno

Crea un archivo `.env` en la raíz del proyecto:

```env
# Configuración del servidor
PORT=8080
GIN_MODE=release

# Configuración de base de datos MySQL
DB_HOST=localhost
DB_PORT=3312
DB_USER=root
DB_PASSWORD=password
DB_NAME=book_prices

# Configuración de web scraping
SCRAPING_TIMEOUT=30
USER_AGENT=BookScraper/1.0
MAX_RETRIES=3
SCRAPING_SOURCES=buscalibre,panamericana
BUSCALIBRE_BASE_URL=https://www.buscalibre.com.co
PANAMERICANA_BASE_URL=https://www.panamericana.com.co

# Configuración de Kafka
KAFKA_BROKER=kafka:9092
KAFKA_TOPIC=recommendation-scrapping
```

### Base de Datos

El servicio utiliza **MySQL** con dos tablas principales:

```sql
-- Tabla principal de precios (historial completo)
CREATE TABLE book_prices (
    id INT AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    price DECIMAL(10,2) NOT NULL,
    source VARCHAR(100) NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    INDEX idx_title (title),
    INDEX idx_price (price),
    INDEX idx_source (source)
);

-- Tabla de libros únicos (precio mínimo por libro)
CREATE TABLE unique_books (
    id INT AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    min_price DECIMAL(10,2) NOT NULL,
    source VARCHAR(100) NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY unique_title (title)
);
```

## 🏗️ Arquitectura del proyecto

```
Back-Web_Scraping/
├── 📁 cmd/
│   └── server/
│       └── main.go            # 🚀 Punto de entrada principal
├── 📁 internal/
│   ├── 📁 config/
│   │   └── config.go          # ⚙️ Configuración y variables de entorno
│   ├── 📁 db/
│   │   └── mysql.go           # 🗄️ Conexión y operaciones de base de datos
│   ├── 📁 handlers/
│   │   └── handlers.go        # 🌐 Controladores HTTP y lógica de negocio
│   ├── 📁 kafka/
│   │   ├── producer.go        # 📤 Productor de eventos Kafka
│   │   └── consumer.go        # 📥 Consumidor de eventos Kafka
│   ├── 📁 models/
│   │   └── book.go           # 📊 Modelos de datos y estructuras
│   └── 📁 scraper/
│       └── scraper.go        # 🕷️ Lógica de web scraping
├── 📁 scripts/
│   ├── init.sql              # 🗃️ Script de inicialización de BD
│   └── create_unique_books_table.sql # 📚 Tabla de libros únicos
├── docker-compose.yml        # 🐳 Configuración Docker
├── Dockerfile               # 📦 Imagen Docker del servicio
├── go.mod                   # 📋 Dependencias Go
└── README.md               # 📖 Este archivo
```

## 🔄 Flujo de Funcionamiento

### 1. Scraping Manual (GET /scrape/:book)
1. Recibe solicitud con título del libro
2. Ejecuta scraping en paralelo en todas las fuentes configuradas
3. Guarda todos los precios encontrados en `book_prices`
4. Calcula el precio mínimo
5. **Envía evento a Kafka** con título y precio mínimo
6. Retorna resultados al cliente

### 2. Procesamiento Asíncrono (Kafka Consumer)
1. **Escucha continuamente** el topic `recommendation-scrapping`
2. **Recibe eventos** con estructura:
   ```json
   {
     "book_title": "Don Quijote",
     "min_price": 45000,
     "source": "web_scraper_service",
     "action": "scraped"
   }
   ```
3. **Actualiza tabla `unique_books`**:
   - Si el libro no existe → lo inserta
   - Si existe y el nuevo precio es menor → lo actualiza
   - Si existe y el nuevo precio es mayor → no hace cambios

### 3. Consulta de Datos
- `/books`: Historial completo de todos los scrapings
- `/unique-books`: Libros únicos con sus precios mínimos actuales

## 🧪 Testing y desarrollo

### **Pruebas de Funcionamiento**

```bash
# 1. Verificar que el servicio está corriendo
curl "http://localhost:8080/health"

# 2. Hacer scraping de un libro específico
curl "http://localhost:8080/scrape/Harry%20Potter"

# 3. Ver todos los precios encontrados
curl "http://localhost:8080/books"

# 4. Ver libros únicos con precio mínimo
curl "http://localhost:8080/unique-books"

# 5. Encolar libro para procesamiento asíncrono
curl -X POST "http://localhost:8080/enqueue" \
  -H "Content-Type: application/json" \
  -d '{"book_title": "El Hobbit"}'
```

### **Monitoreo de Logs**

El servicio proporciona logs detallados:

```bash
# Logs del consumer de Kafka
🎧 Iniciando consumer de Kafka para topic: recommendation-scrapping
✅ Consumer suscrito exitosamente al topic: recommendation-scrapping
📥 Mensaje recibido de Kafka - Topic: recommendation-scrapping
📚 Procesando evento - Libro: 'Don Quijote', Precio: $45000
✅ Libro único guardado/actualizado exitosamente

# Logs de scraping
🕷️ Iniciando scraping para libro: 'Harry Potter'
📊 Buscalibre: Encontrados 3 precios para 'Harry Potter'
📊 Panamericana: Encontrados 2 precios para 'Harry Potter'
💰 Precio mínimo encontrado: $35000 en Buscalibre
```

## 🛡️ Consideraciones de Seguridad y Rendimiento

### **Web Scraping Responsable**
- User-Agent identificable: `BookScraper/1.0`
- Timeouts configurables para evitar bloqueos
- Sistema de reintentos limitado (máximo 3 intentos)
- Respeto por la estructura de robots.txt

### **Optimización de Base de Datos**
- Índices en campos de búsqueda frecuente (título, precio, fuente)
- Constraint de unicidad en tabla `unique_books`
- Timestamps automáticos para auditoría
- Conexiones pooled para mejor rendimiento

### **Tolerancia a Fallos**
- Manejo de errores en scraping sin afectar otros sitios
- Consumer de Kafka con reconexión automática
- Health checks para monitoreo externo
- Logs estructurados para debugging

## 🐛 Solución de problemas

### Error: "Database connection failed"
```bash
# Verificar que MySQL está corriendo
docker-compose ps mysql-scraper

# Verificar configuración de BD
echo $DB_HOST $DB_PORT $DB_USER
```

### Error: "Kafka producer failed"
```bash
# Verificar conexión con Kafka
docker-compose ps kafka

# Verificar configuración de Kafka
echo $KAFKA_BROKER $KAFKA_TOPIC
```

### Puerto 8080 ocupado
```bash
# Cambiar puerto en .env o docker-compose.yml
PORT=8081

# O detener el proceso que usa el puerto
sudo lsof -ti:8080 | xargs kill -9
```

### Scraping devuelve resultados vacíos
```bash
# Verificar conectividad a sitios web
curl -I https://www.buscalibre.com.co

# Revisar logs del servicio para errores específicos
docker-compose logs book-scraper
```

## 🔗 Integración con otros servicios

Este servicio se integra con la arquitectura de microservicios de BookReview:

- **Back-recommendations**: Recibe eventos via Kafka para generar recomendaciones basadas en precios
- **Frontend**: Consume endpoints REST para mostrar precios actualizados
- **Middleware-GraphQL**: Puede consumir los endpoints via GraphQL queries

### Ejemplo de integración via GraphQL:
```graphql
query GetBookPrices($bookTitle: String!) {
  bookPrices(title: $bookTitle) {
    title
    minPrice
    source
    updatedAt
  }
}
```

## 📞 Soporte

Si encuentras algún problema o tienes preguntas:

1. Revisa los logs del servicio: `docker-compose logs book-scraper`
2. Verifica la documentación de Go Colly: https://go-colly.org/
3. Consulta la documentación de Gin: https://gin-gonic.com/
4. Abre un issue en el repositorio del proyecto

## 🤝 Contribuciones

1. Fork el proyecto
2. Crea una rama para tu feature (`git checkout -b feature/nueva-fuente-scraping`)
3. Commit tus cambios (`git commit -am 'Agrega scraping de nueva librería'`)
4. Push a la rama (`git push origin feature/nueva-fuente-scraping`)
5. Abre un Pull Request

## 📄 Licencia

Este proyecto está bajo la Licencia MIT - ver el archivo [LICENSE](LICENSE) para más detalles.

---

**Desarrollado con ❤️ para BookReview Platform**