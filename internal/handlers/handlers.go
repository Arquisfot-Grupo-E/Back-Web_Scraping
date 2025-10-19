package handlers

import (
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/config"
	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/db"
	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/models"
	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/scraper"
	"github.com/gin-gonic/gin"

	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/kafka"
)

// Handlers contiene todas las dependencias necesarias para los controladores HTTP
type Handlers struct {
	db      *db.Database     // Conexión a la base de datos
	scraper *scraper.Scraper // Instancia del scraper
	config  *config.Config   // Configuración de la aplicación
	producer *kafka.KafkaProducer // 👈 nuevo campo
}

// NewHandlers crea una nueva instancia de handlers con todas las dependencias
func NewHandlers(database *db.Database, scraperInstance *scraper.Scraper, cfg *config.Config, producer *kafka.KafkaProducer) *Handlers {
	return &Handlers{
		db:       database,
		scraper:  scraperInstance,
		config:   cfg,
		producer: producer,
	}
}

// HealthCheck verifica el estado del servicio y sus dependencias
// GET /health
func (h *Handlers) HealthCheck(c *gin.Context) {
	log.Println("🔍 Health check solicitado")

	// Verificar estado de la base de datos
	dbStatus := "connected"
	if err := h.db.Ping(); err != nil {
		log.Printf("❌ Error conectando a BD: %v", err)
		dbStatus = "disconnected"
	}

	// Preparar respuesta de health check
	response := models.HealthResponse{
		Status:    "healthy",
		Service:   "book-scraper-service",
		Timestamp: time.Now().Format(time.RFC3339),
		Database:  dbStatus,
	}

	// Si la BD está desconectada, marcar servicio como unhealthy
	if dbStatus == "disconnected" {
		response.Status = "unhealthy"
		c.JSON(http.StatusServiceUnavailable, response)
		return
	}

	log.Println("✅ Health check OK")
	c.JSON(http.StatusOK, response)
}

// ScrapeBook hace scraping inmediato de un libro y guarda los resultados
// GET /scrape/:book
func (h *Handlers) ScrapeBook(c *gin.Context) {
	bookName := c.Param("book")
	if bookName == "" {
		log.Println("❌ Parámetro 'book' requerido pero no proporcionado")
		c.JSON(http.StatusBadRequest, gin.H{"error": "book parameter is required"})
		return
	}

	log.Printf("🚀 Iniciando scraping para: '%s'", bookName)

	// Hacer scraping en paralelo en ambas fuentes
	result := h.scraper.ScrapeBookPrices(bookName)

	// Guardar todos los precios encontrados en la base de datos
	savedCount := 0
	for _, price := range result.Prices {
		if err := h.db.InsertBookPrice(&price); err != nil {
			log.Printf("❌ Error guardando precio de %s: %v", price.Source, err)
		} else {
			log.Printf("💾 Precio guardado: %s - $%.0f", price.Source, price.Price)
			savedCount++
		}
	}
	// Enviar evento a Kafka automáticamente después del scraping
	log.Printf("📨 Enviando evento a Kafka para: '%s'", bookName)
	kafkaEvent := map[string]string{
		"book_title": bookName,
		"source":     "web_scraper_service",
		"action":     "scraped",
	}

	if err := h.producer.Publish(kafkaEvent); err != nil {
		log.Printf("❌ Error enviando evento a Kafka: %v", err)
	} else {
		log.Printf("✅ Evento enviado a Kafka correctamente para '%s'", bookName)
	}
	// Agregar información sobre cuántos precios se guardaron
	if len(result.Prices) > 0 {
		result.Message += fmt.Sprintf(" (%d precios guardados en BD)", savedCount)
	}

	log.Printf("✅ Scraping completado para '%s': %s", bookName, result.Status)
	c.JSON(http.StatusOK, result)
}

// GetAllBooks obtiene todos los libros guardados en la base de datos
// GET /books
func (h *Handlers) GetAllBooks(c *gin.Context) {
	log.Println("📚 Obteniendo todos los libros de la BD")

	books, err := h.db.GetAllBooks()
	if err != nil {
		log.Printf("❌ Error obteniendo libros: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	log.Printf("✅ Se encontraron %d registros de precios", len(books))

	response := gin.H{
		"books":     books,
		"count":     len(books),
		"timestamp": time.Now().Format(time.RFC3339),
	}

	c.JSON(http.StatusOK, response)
}

// EnqueueBook encola un libro para procesamiento asíncrono (Kafka)
// POST /enqueue
func (h *Handlers) EnqueueBook(c *gin.Context) {
	var request models.EnqueueRequest

	if err := c.ShouldBindJSON(&request); err != nil {
		log.Printf("❌ Error parseando JSON: %v", err)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	if request.Book == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "book field is required"})
		return
	}

	log.Printf("📨 Encolando libro para Kafka: '%s'", request.Book)

	event := map[string]string{
		"book_title": request.Book,
		"source":     "web_scraper_service",
	}

	if err := h.producer.Publish(event); err != nil {
		log.Printf("❌ Error publicando en Kafka: %v", err)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "error sending message to Kafka"})
		return
	}

	response := models.EnqueueResponse{
		Message: "Book sent to Kafka for asynchronous processing",
		Book:    request.Book,
		Status:  "queued",
	}

	log.Printf("✅ Libro '%s' enviado a Kafka correctamente", request.Book)
	c.JSON(http.StatusOK, response)
}
