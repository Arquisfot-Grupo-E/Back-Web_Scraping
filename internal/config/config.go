package config

import (
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

// Config estructura que contiene toda la configuración de la aplicación
type Config struct {
	// Configuración del servidor HTTP
	Port    string
	GinMode string

	// Configuración de base de datos MySQL
	DBHost     string
	DBPort     string
	DBUser     string
	DBPassword string
	DBName     string

	// Configuración de web scraping
	ScrapingTimeout     int      // Tiempo máximo de espera por petición (segundos)
	UserAgent           string   // Identificación del scraper hacia las páginas web
	MaxRetries          int      // Número máximo de reintentos por sitio web
	ScrapingSources     []string // Fuentes de scraping habilitadas
	BuscalibreBaseURL   string   // URL base de Buscalibre
	PanamericanaBaseURL string   // URL base de Panamericana
	CasaDelLibroBaseURL string   // URL base de Casa del Libro
}

// LoadConfig carga la configuración desde variables de entorno y archivo .env
func LoadConfig() *Config {
	// Cargar archivo .env si existe (no es obligatorio)
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found, using system environment variables")
	}

	// Convertir valores string a entero con valores por defecto seguros
	timeout, _ := strconv.Atoi(getEnv("SCRAPING_TIMEOUT", "30"))
	maxRetries, _ := strconv.Atoi(getEnv("MAX_RETRIES", "3"))

	return &Config{
		// Configuración del servidor
		Port:    getEnv("PORT", "8080"),
		GinMode: getEnv("GIN_MODE", "debug"),

		// Configuración de base de datos
		DBHost:     getEnv("DB_HOST", "localhost"),
		DBPort:     getEnv("DB_PORT", "3312"),
		DBUser:     getEnv("DB_USER", "root"),
		DBPassword: getEnv("DB_PASSWORD", "password"),
		DBName:     getEnv("DB_NAME", "book_prices"),

		// Configuración de scraping
		ScrapingTimeout:     timeout,
		UserAgent:           getEnv("USER_AGENT", "BookScraper/1.0"),
		MaxRetries:          maxRetries,
		ScrapingSources:     parseScrapingSources(getEnv("SCRAPING_SOURCES", "buscalibre,panamericana,casadellibro")),
		BuscalibreBaseURL:   getEnv("BUSCALIBRE_BASE_URL", "https://www.buscalibre.com.co"),
		PanamericanaBaseURL: getEnv("PANAMERICANA_BASE_URL", "https://www.panamericana.com.co"),
		CasaDelLibroBaseURL: getEnv("CASADELLIBRO_BASE_URL", "https://www.casadellibro.com.co"),
	}
}

// getEnv obtiene una variable de entorno o retorna un valor por defecto
// Esto permite que la aplicación funcione aunque no tengamos archivo .env
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// GetDBConnectionString retorna la cadena de conexión para MySQL
// Formato: usuario:password@tcp(host:puerto)/basededatos?parseTime=true
func (c *Config) GetDBConnectionString() string {
	return c.DBUser + ":" + c.DBPassword + "@tcp(" + c.DBHost + ":" + c.DBPort + ")/" + c.DBName + "?parseTime=true"
}

// parseScrapingSources convierte la string de fuentes separadas por comas en un slice
// Ejemplo: "buscalibre,panamericana" -> ["buscalibre", "panamericana"]
func parseScrapingSources(sources string) []string {
	if sources == "" {
		return []string{}
	}

	// Dividir por comas y limpiar espacios
	parts := strings.Split(sources, ",")
	result := make([]string, 0, len(parts))

	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
