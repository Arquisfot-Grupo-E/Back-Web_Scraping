package models

import "time"

// BookPrice representa el precio de un libro de una fuente específica
// Esta estructura mapea directamente con la tabla MySQL 'book_prices'
type BookPrice struct {
	ID        int       `json:"id" db:"id"`                 // ID único en la base de datos
	Title     string    `json:"title" db:"title"`           // Título del libro
	Price     float64   `json:"price" db:"price"`           // Precio del libro
	Source    string    `json:"source" db:"source"`         // Fuente (Casa del Libro, Buscalibre, etc.)
	ScrapedAt time.Time `json:"scraped_at" db:"scraped_at"` // Cuándo se hizo el scraping
}

// BookResult representa el resultado completo del scraping de un libro
// Contiene todos los precios encontrados en diferentes fuentes
type BookResult struct {
	BookTitle string      `json:"book_title"` // Título del libro buscado
	Prices    []BookPrice `json:"prices"`     // Array con precios de diferentes fuentes
	Status    string      `json:"status"`     // "success", "partial", "error"
	Message   string      `json:"message"`    // Mensaje adicional o error
}

// EnqueueRequest representa la petición para encolar un libro para scraping asíncrono
type EnqueueRequest struct {
	Book string `json:"book" binding:"required"` // Nombre del libro (campo obligatorio)
}

// EnqueueResponse representa la respuesta del endpoint de encolado
type EnqueueResponse struct {
	Message string `json:"message"` // Mensaje de confirmación
	Book    string `json:"book"`    // Libro que se encoló
	Status  string `json:"status"`  // "queued", "error"
}

// HealthResponse representa la respuesta del health check
type HealthResponse struct {
	Status    string `json:"status"`    // "healthy", "unhealthy"
	Service   string `json:"service"`   // "book-scraper-service"
	Timestamp string `json:"timestamp"` // Timestamp actual
	Database  string `json:"database"`  // Estado de la base de datos
}

// KafkaEvent representa el evento que se envía a Kafka
type KafkaEvent struct {
	BookTitle string  `json:"book_title"` // Título del libro
	MinPrice  float64 `json:"min_price"`  // Menor precio encontrado
	Source    string  `json:"source"`     // Fuente del evento (web_scraper_service)
	Action    string  `json:"action"`     // Acción realizada (scraped)
}

// UniqueBook representa un libro único con su menor precio
// Para la tabla de libros únicos en la BD
type UniqueBook struct {
	ID        int       `json:"id" db:"id"`                 // ID único en la base de datos
	Title     string    `json:"title" db:"title"`           // Título del libro
	MinPrice  float64   `json:"min_price" db:"min_price"`   // Menor precio encontrado
	Source    string    `json:"source" db:"source"`         // Fuente del menor precio
	UpdatedAt time.Time `json:"updated_at" db:"updated_at"` // Última actualización
}
