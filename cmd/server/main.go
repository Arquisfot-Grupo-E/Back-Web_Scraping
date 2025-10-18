package main

import (
	"log"

	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/config"
	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/db"
	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/handlers"
	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/scraper"
	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("📚 Iniciando Book Scraper Service...")

	// 1. Cargar configuración desde .env y variables de entorno
	log.Println("⚙️ Cargando configuración...")
	cfg := config.LoadConfig()
	log.Printf("✅ Configuración cargada - Puerto: %s, Fuentes: %v", cfg.Port, cfg.ScrapingSources)

	// 2. Conectar a base de datos MySQL
	log.Println("🗄️ Conectando a base de datos...")
	database, err := db.NewDatabase(cfg)
	if err != nil {
		log.Fatalf("❌ Error fatal conectando a BD: %v", err)
	}
	defer database.Close()
	log.Println("✅ Conexión a base de datos establecida")

	// 3. Inicializar scraper con configuración
	log.Println("🕷️ Inicializando web scraper...")
	scraperInstance := scraper.NewScraper(cfg)
	log.Printf("✅ Scraper configurado para fuentes: %v", cfg.ScrapingSources)

	// 4. Inicializar handlers con dependencias
	log.Println("🎯 Configurando handlers...")
	h := handlers.NewHandlers(database, scraperInstance, cfg)

	// 5. Configurar Gin router y middleware
	gin.SetMode(cfg.GinMode)
	router := gin.Default()

	// Middleware global
	router.Use(gin.Logger())          // Logging de requests
	router.Use(gin.Recovery())        // Recovery de panics
	router.Use(func(c *gin.Context) { // CORS básico
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 6. Configurar rutas de la API
	log.Println("🛣️ Configurando rutas...")

	// Health check
	router.GET("/health", h.HealthCheck)

	// Scraping endpoints
	router.GET("/scrape/:book", h.ScrapeBook) // Scraping inmediato
	router.GET("/books", h.GetAllBooks)       // Listar todos los libros guardados

	// Async processing endpoint (preparado para persona B)
	router.POST("/enqueue", h.EnqueueBook) // Encolar para procesamiento asíncrono

	// Ruta de información
	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"service": "Book Scraper Service",
			"version": "1.0.0",
			"status":  "running",
			"endpoints": gin.H{
				"health":  "GET /health",
				"scrape":  "GET /scrape/:book",
				"books":   "GET /books",
				"enqueue": "POST /enqueue",
			},
		})
	})

	log.Printf("✅ Rutas configuradas correctamente")

	// 7. Iniciar servidor HTTP
	serverAddr := ":" + cfg.Port
	log.Printf("🚀 Iniciando servidor en http://localhost%s", serverAddr)
	log.Printf("📖 Documentación disponible en http://localhost%s/", serverAddr)

	if err := router.Run(serverAddr); err != nil {
		log.Fatalf("❌ Error fatal iniciando servidor: %v", err)
	}
}
