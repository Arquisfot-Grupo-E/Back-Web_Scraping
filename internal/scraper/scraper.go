package scraper

import (
	"encoding/json"
	"fmt"
	"log"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/config"
	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/models"
	"github.com/gocolly/colly/v2"
	"github.com/gocolly/colly/v2/debug"
)

type Scraper struct {
	config *config.Config
}

// Estructuras para parsear JSON-LD de Panamericana
type PanamericanaProduct struct {
	Context          string                 `json:"@context"`
	Type             string                 `json:"@type"`
	ItemListElements []PanamericanaListItem `json:"itemListElement"`
}

type PanamericanaListItem struct {
	Type     string           `json:"@type"`
	Position int              `json:"position"`
	Item     PanamericanaItem `json:"item"`
}

type PanamericanaItem struct {
	Context     string              `json:"@context"`
	Type        string              `json:"@type"`
	ID          string              `json:"@id"`
	Name        string              `json:"name"`
	Brand       PanamericanaBrand   `json:"brand"`
	Offers      []PanamericanaOffer `json:"offers"` // Array de ofertas
	Description string              `json:"description"`
}

type PanamericanaBrand struct {
	Type string `json:"@type"`
	Name string `json:"name"`
}

type PanamericanaOffer struct {
	Type          string             `json:"@type"`
	Price         float64            `json:"price"` // Precio individual
	PriceCurrency string             `json:"priceCurrency"`
	Availability  string             `json:"availability"`
	Seller        PanamericanaSeller `json:"seller"`
}

type PanamericanaSeller struct {
	Type string `json:"@type"`
	Name string `json:"name"`
}

// NewScraper crea una nueva instancia del scraper
func NewScraper(cfg *config.Config) *Scraper {
	return &Scraper{config: cfg}
}

// ScrapeBookPrices hace scraping de precios de un libro en múltiples fuentes configuradas
func (s *Scraper) ScrapeBookPrices(bookTitle string) *models.BookResult {
	result := &models.BookResult{
		BookTitle: bookTitle,
		Prices:    []models.BookPrice{},
		Status:    "success",
		Message:   "",
	}

	// Obtener número de fuentes activas
	numSources := len(s.config.ScrapingSources)
	if numSources == 0 {
		result.Status = "error"
		result.Message = "No hay fuentes de scraping configuradas"
		return result
	}

	// Canales para recoger resultados de scrapers concurrentes
	priceChan := make(chan models.BookPrice, numSources)
	errorChan := make(chan error, numSources)

	// Lanzar scrapers concurrentes según configuración
	for _, source := range s.config.ScrapingSources {
		switch strings.ToLower(source) {
		case "buscalibre":
			go s.scrapeBuscalibre(bookTitle, priceChan, errorChan)
		case "panamericana":
			go s.scrapePanamericana(bookTitle, priceChan, errorChan)
		default:
			log.Printf("⚠️  Fuente de scraping no reconocida: %s", source)
			go func() { errorChan <- fmt.Errorf("fuente no reconocida: %s", source) }()
		}
	}

	// Recoger resultados con timeout
	timeout := time.After(time.Duration(s.config.ScrapingTimeout) * time.Second)
	receivedCount := 0

	for receivedCount < numSources {
		select {
		case price := <-priceChan:
			if price.Price > 0 {
				result.Prices = append(result.Prices, price)
				log.Printf("✅ Precio encontrado en %s: $%.2f", price.Source, price.Price)
			}
			receivedCount++
		case err := <-errorChan:
			log.Printf("❌ Error en scraping: %v", err)
			receivedCount++
		case <-timeout:
			log.Printf("⏰ Timeout alcanzado para scraping de '%s'", bookTitle)
			receivedCount = numSources // Forzar salida
		}
	}

	// Determinar status final
	s.setResultStatus(result, numSources)
	return result
}

// setResultStatus determina el status y mensaje final del resultado
func (s *Scraper) setResultStatus(result *models.BookResult, expectedSources int) {
	foundPrices := len(result.Prices)

	if foundPrices == 0 {
		result.Status = "error"
		result.Message = "No se encontraron precios en ninguna fuente"
	} else if foundPrices < expectedSources {
		result.Status = "partial"
		result.Message = fmt.Sprintf("Precios encontrados en %d de %d fuentes", foundPrices, expectedSources)
	} else {
		result.Status = "success"
		result.Message = fmt.Sprintf("Precios encontrados en todas las fuentes (%d)", foundPrices)
	}
}

// scrapeBuscalibre realiza scraping en Buscalibre Colombia
func (s *Scraper) scrapeBuscalibre(bookTitle string, priceChan chan<- models.BookPrice, errorChan chan<- error) {
	defer func() {
		if r := recover(); r != nil {
			errorChan <- fmt.Errorf("panic en Buscalibre scraper: %v", r)
		}
	}()

	// Crear collector con configuración
	c := s.createCollector()

	var minPrice float64 = 0
	var bestTitle string
	found := false

	// Configurar callback específico para Buscalibre
	c.OnHTML(".box-producto", func(e *colly.HTMLElement) {
		// Extraer título del libro
		title := strings.TrimSpace(e.ChildText("h3.nombre"))

		// Extraer precio actual (strong dentro de .box-precios)
		priceText := strings.TrimSpace(e.ChildText(".box-precios strong"))

		log.Printf("� Buscalibre encontró libro - Título: '%s', Precio: '%s'", title, priceText)

		if title != "" && priceText != "" && s.matchesTitle(title, bookTitle) {
			if price := s.parsePrice(priceText); price > 0 {
				if minPrice == 0 || price < minPrice {
					minPrice = price
					bestTitle = title
					found = true
					log.Printf("✅ Buscalibre - Mejor precio: $%.0f - %s", price, title)
				}
			}
		}
	})

	// URL de búsqueda en Buscalibre (formato específico)
	encodedTitle := url.QueryEscape(bookTitle)
	searchURL := fmt.Sprintf("%s/libros/search/?q=%s", s.config.BuscalibreBaseURL, encodedTitle)

	log.Printf("🔗 Buscando en Buscalibre: %s", searchURL)

	if err := c.Visit(searchURL); err != nil {
		errorChan <- fmt.Errorf("error visitando Buscalibre: %v", err)
		return
	}

	if found && minPrice > 0 {
		finalPrice := models.BookPrice{
			Title:     bestTitle,
			Price:     minPrice,
			Source:    "Buscalibre",
			ScrapedAt: time.Now(),
		}
		log.Printf("🎉 Buscalibre - Precio final: $%.0f para '%s'", minPrice, bestTitle)
		priceChan <- finalPrice
	} else {
		errorChan <- fmt.Errorf("no se encontró '%s' en Buscalibre", bookTitle)
	}
}

// scrapePanamericana realiza scraping en Panamericana usando JSON-LD
func (s *Scraper) scrapePanamericana(bookTitle string, priceChan chan<- models.BookPrice, errorChan chan<- error) {
	defer func() {
		if r := recover(); r != nil {
			errorChan <- fmt.Errorf("panic en Panamericana scraper: %v", r)
		}
	}()

	// Crear collector con configuración
	c := s.createCollector()

	var minPrice float64 = 0
	var bestProduct PanamericanaItem
	found := false

	// Buscar JSON-LD estructurado en la página
	c.OnHTML("script[type='application/ld+json']", func(e *colly.HTMLElement) {
		jsonText := e.Text
		log.Printf("🔍 JSON-LD encontrado en Panamericana")

		var product PanamericanaProduct
		if err := json.Unmarshal([]byte(jsonText), &product); err != nil {
			log.Printf("⚠️ Error parseando JSON-LD de Panamericana: %v", err)
			// Intentar parsear como array directo de productos
			var items []PanamericanaItem
			if err2 := json.Unmarshal([]byte(jsonText), &items); err2 != nil {
				log.Printf("⚠️ Tampoco se pudo parsear como array: %v", err2)
				return
			}
			// Si es array directo, procesarlo
			for _, item := range items {
				s.processPanamericanaItem(item, bookTitle, &minPrice, &bestProduct, &found)
			}
			return
		}

		// Buscar en itemListElement si existe
		if len(product.ItemListElements) > 0 {
			for _, listItem := range product.ItemListElements {
				s.processPanamericanaItem(listItem.Item, bookTitle, &minPrice, &bestProduct, &found)
			}
		}
	})

	// Construir URL de búsqueda para Panamericana
	// Formato: https://www.panamericana.com.co/titulo-libro?_q=titulo-libro&map=ft
	formattedTitle := strings.ToLower(strings.ReplaceAll(bookTitle, " ", "%20"))
	encodedQuery := url.QueryEscape(bookTitle)
	searchURL := fmt.Sprintf("%s/%s?_q=%s&map=ft",
		s.config.PanamericanaBaseURL,
		formattedTitle,
		encodedQuery)

	log.Printf("🌐 Panamericana URL: %s", searchURL)

	log.Printf("🔗 Buscando en Panamericana: %s", searchURL)

	if err := c.Visit(searchURL); err != nil {
		errorChan <- fmt.Errorf("error visitando Panamericana: %v", err)
		return
	}

	if found && minPrice > 0 {
		finalPrice := models.BookPrice{
			Title:     bestProduct.Name,
			Price:     minPrice,
			Source:    "Panamericana",
			ScrapedAt: time.Now(),
		}
		log.Printf("🎉 Panamericana - Precio final: $%.0f para '%s'", minPrice, bestProduct.Name)
		priceChan <- finalPrice
	} else {
		errorChan <- fmt.Errorf("no se encontró '%s' en Panamericana", bookTitle)
	}
}

// createCollector crea un collector de colly con la configuración común
func (s *Scraper) createCollector() *colly.Collector {
	c := colly.NewCollector(
		colly.Debugger(&debug.LogDebugger{}),
	)

	// Configurar User-Agent
	c.UserAgent = s.config.UserAgent

	// Configurar timeout
	c.SetRequestTimeout(time.Duration(s.config.ScrapingTimeout) * time.Second)

	// Límite de requests por segundo para ser respetuosos
	c.Limit(&colly.LimitRule{
		DomainGlob:  "*",
		Parallelism: 1,
		Delay:       1 * time.Second,
	})

	return c
}

// matchesTitle verifica si un título encontrado coincide con el buscado
func (s *Scraper) matchesTitle(foundTitle, searchTitle string) bool {
	found := strings.ToLower(strings.TrimSpace(foundTitle))
	search := strings.ToLower(strings.TrimSpace(searchTitle))

	log.Printf("🔍 Comparando: '%s' vs '%s'", found, search)

	// Coincidencia exacta
	if found == search {
		log.Printf("✅ Coincidencia exacta encontrada!")
		return true
	}

	// Coincidencia parcial (contiene)
	match := strings.Contains(found, search) || strings.Contains(search, found)
	if match {
		log.Printf("✅ Coincidencia parcial encontrada!")
	} else {
		log.Printf("❌ No hay coincidencia")
	}
	return match
}

// parsePrice extrae el valor numérico del precio desde una string
// Maneja formatos colombianos como: $49.000, $49,000, 49000, etc.
func (s *Scraper) parsePrice(priceText string) float64 {
	if priceText == "" {
		return 0
	}

	// Limpiar el texto del precio
	cleaned := strings.TrimSpace(priceText)
	cleaned = strings.ReplaceAll(cleaned, "$", "")
	cleaned = strings.ReplaceAll(cleaned, "COP", "")
	cleaned = strings.ReplaceAll(cleaned, " ", "")

	// Manejar formato colombiano: 49.000 o 49,000 (miles separados)
	if strings.Contains(cleaned, ".") && len(strings.Split(cleaned, ".")) == 2 {
		parts := strings.Split(cleaned, ".")
		if len(parts[1]) == 3 { // Es separador de miles: 49.000
			cleaned = strings.ReplaceAll(cleaned, ".", "")
		}
	}

	// Quitar comas (separadores de miles)
	cleaned = strings.ReplaceAll(cleaned, ",", "")

	// Convertir a float
	if price, err := strconv.ParseFloat(cleaned, 64); err == nil {
		if price > 0 {
			log.Printf("💰 Precio parseado: '%s' -> %.0f", priceText, price)
			return price
		}
	}

	log.Printf("⚠️ No se pudo parsear precio: '%s'", priceText)
	return 0
}

// processPanamericanaItem procesa un item individual de Panamericana
func (s *Scraper) processPanamericanaItem(item PanamericanaItem, bookTitle string, minPrice *float64, bestProduct *PanamericanaItem, found *bool) {
	log.Printf("📚 Panamericana encontró libro - Título: '%s'", item.Name)

	if !s.matchesTitle(item.Name, bookTitle) {
		return
	}

	log.Printf("✅ Panamericana - Match encontrado! Procesando: %s", item.Name)

	// Buscar el precio más barato en las ofertas
	for _, offer := range item.Offers {
		price := offer.Price
		log.Printf("🏷️  Oferta encontrada - Precio: $%.0f, Vendedor: %s, Disponibilidad: %s",
			price, offer.Seller.Name, offer.Availability)

		if price > 0 && (*minPrice == 0 || price < *minPrice) {
			*minPrice = price
			*bestProduct = item
			*found = true
			log.Printf("✅ Panamericana - Nuevo mejor precio: $%.0f - %s", price, item.Name)
		}
	}
}
