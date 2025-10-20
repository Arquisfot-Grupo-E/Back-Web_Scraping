package main

import (
	"log"

	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/config"
	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/db"
	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/handlers"
	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/kafka"
	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/scraper"
	"github.com/gin-gonic/gin"
)

func main() {
	log.Println("📚 Iniciando Book Scraper Service...")

	// 1. Cargar configuración
	log.Println("⚙️ Cargando configuración...")
	cfg := config.LoadConfig()
	log.Printf("✅ Configuración cargada - Puerto: %s, Fuentes: %v", cfg.Port, cfg.ScrapingSources)

	// 2. Conectar a base de datos
	log.Println("🗄️ Conectando a base de datos...")
	database, err := db.NewDatabase(cfg)
	if err != nil {
		log.Fatalf("❌ Error fatal conectando a BD: %v", err)
	}
	defer database.Close()
	log.Println("✅ Conexión a base de datos establecida")

	// 3. Inicializar scraper
	log.Println("🕷️ Inicializando web scraper...")
	scraperInstance := scraper.NewScraper(cfg)
	log.Printf("✅ Scraper configurado para fuentes: %v", cfg.ScrapingSources)

	// 4. Inicializar Kafka Producer
	log.Println("📡 Conectando con Kafka...")
	producer, err := kafka.NewKafkaProducer(cfg.KafkaBroker, cfg.KafkaTopic)
	if err != nil {
		log.Fatalf("❌ No se pudo crear Kafka producer: %v", err)
	}
	defer producer.Close()
	log.Println("✅ Conexión con Kafka establecida correctamente")

	// 5. Inicializar Kafka Consumer
	log.Println("🎧 Inicializando Kafka Consumer...")
	consumer, err := kafka.NewKafkaConsumer(cfg.KafkaBroker, cfg.KafkaTopic, "book-scraper-consumer-group", database)
	if err != nil {
		log.Fatalf("❌ No se pudo crear Kafka consumer: %v", err)
	}
	defer consumer.Close()
	log.Println("✅ Kafka Consumer inicializado correctamente")

	// Iniciar el consumer en una goroutine separada
	go func() {
		log.Println("🚀 Iniciando Kafka Consumer en goroutine...")
		if err := consumer.Start(); err != nil {
			log.Printf("❌ Error en Kafka Consumer: %v", err)
		}
	}()

	// 6. Inicializar handlers con dependencias
	h := handlers.NewHandlers(database, scraperInstance, cfg, producer)


	// 7. Configurar router y middlewares
	gin.SetMode(cfg.GinMode)
	router := gin.Default()

	router.Use(gin.Logger())
	router.Use(gin.Recovery())
	router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type")
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		c.Next()
	})

	// 8. Configurar rutas
	log.Println("🛣️ Configurando rutas...")

	router.GET("/health", h.HealthCheck)
	router.GET("/scrape/:book", h.ScrapeBook)
	router.GET("/books", h.GetAllBooks)
	router.GET("/unique-books", h.GetAllUniqueBooks)
	router.POST("/enqueue", h.EnqueueBook)

	router.GET("/", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"service": "Book Scraper Service",
			"version": "1.0.0",
			"status":  "running",
			"endpoints": gin.H{
				"health":       "GET /health",
				"scrape":       "GET /scrape/:book",
				"books":        "GET /books",
				"unique-books": "GET /unique-books",
				"enqueue":      "POST /enqueue",
			},
		})
	})

	log.Printf("✅ Rutas configuradas correctamente")

	// 9. Iniciar servidor HTTP
	serverAddr := ":" + cfg.Port
	log.Printf("🚀 Iniciando servidor en http://localhost%s", serverAddr)
	log.Printf("📖 Documentación disponible en http://localhost%s/", serverAddr)
	log.Printf("🎧 Kafka Consumer ejecutándose en background...")

	if err := router.Run(serverAddr); err != nil {
		log.Fatalf("❌ Error fatal iniciando servidor: %v", err)
	}
}
