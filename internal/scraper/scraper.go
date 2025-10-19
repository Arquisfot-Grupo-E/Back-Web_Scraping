package scraper

import (
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

// Las estructuras de Panamericana han sido removidas ya que usamos scraping HTML directo

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
		case "casadellibro":
			go s.scrapeCasaDelLibro(bookTitle, priceChan, errorChan)
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

// scrapePanamericana realiza scraping en Panamericana usando HTML
func (s *Scraper) scrapePanamericana(bookTitle string, priceChan chan<- models.BookPrice, errorChan chan<- error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ PANIC en Panamericana scraper: %v", r)
			errorChan <- fmt.Errorf("panic en Panamericana scraper: %v", r)
		}
	}()

	log.Printf("🏪 INICIANDO scraping de Panamericana para: '%s'", bookTitle)

	// Crear collector con configuración
	c := s.createCollector()

	var minPrice float64 = 0
	var bestTitle string
	found := false
	elementsFound := 0

	// Usar HTML scraping en lugar de JSON-LD que no funciona consistentemente
	c.OnHTML(".vtex-product-summary-2-x-container, .vtex-search-result-3-x-galleryItem, .vtex-product-summary, .product-item", func(e *colly.HTMLElement) {
		elementsFound++
		log.Printf("🔍 Panamericana - Elemento %d encontrado", elementsFound)

		// Extraer título desde varios selectores posibles
		title := strings.TrimSpace(e.ChildText(".vtex-product-summary-2-x-productBrand"))
		if title == "" {
			title = strings.TrimSpace(e.ChildText(".vtex-store-components-3-x-productBrandName"))
			if title == "" {
				title = strings.TrimSpace(e.ChildText("h3"))
				if title == "" {
					title = strings.TrimSpace(e.ChildText("a"))
					if title == "" {
						title = strings.TrimSpace(e.ChildText(".product-name"))
					}
				}
			}
		}

		// Extraer precio desde varios selectores posibles
		priceText := strings.TrimSpace(e.ChildText(".vtex-product-price-1-x-sellingPrice"))
		if priceText == "" {
			priceText = strings.TrimSpace(e.ChildText(".vtex-store-components-3-x-sellingPrice"))
			if priceText == "" {
				priceText = strings.TrimSpace(e.ChildText(".price"))
				if priceText == "" {
					// Buscar en cualquier elemento que contenga $
					e.ForEach("*", func(i int, elem *colly.HTMLElement) {
						text := strings.TrimSpace(elem.Text)
						if strings.Contains(text, "$") && (strings.Contains(text, "000") || strings.Contains(text, ".")) && len(text) < 20 {
							priceText = text
						}
					})
				}
			}
		}

		log.Printf("📚 Panamericana encontró producto - Título: '%s', Precio: '%s'", title, priceText)

		if title != "" && priceText != "" && s.matchesTitle(title, bookTitle) {
			if price := s.parsePrice(priceText); price > 0 {
				if minPrice == 0 || price < minPrice {
					minPrice = price
					bestTitle = title
					found = true
					log.Printf("✅ Panamericana - Mejor precio: $%.0f - %s", price, title)
				}
			}
		} else if title == "" {
			log.Printf("⚠️ Panamericana - Elemento sin título")
		} else if priceText == "" {
			log.Printf("⚠️ Panamericana - Elemento sin precio: '%s'", title)
		}
	})

	// También intentar con la búsqueda simple de Panamericana
	searchURL := fmt.Sprintf("%s/search/?_query=%s", s.config.PanamericanaBaseURL, url.QueryEscape(bookTitle))

	log.Printf("🌐 Panamericana URL: %s", searchURL)
	log.Printf("🔗 Buscando en Panamericana: %s", searchURL)

	// Callback para debug
	c.OnResponse(func(r *colly.Response) {
		log.Printf("✅ Panamericana - Respuesta HTTP %d recibida, tamaño: %d bytes", r.StatusCode, len(r.Body))
	})

	c.OnError(func(r *colly.Response, err error) {
		log.Printf("❌ Error HTTP en Panamericana (status %d): %v", r.StatusCode, err)
	})

	if err := c.Visit(searchURL); err != nil {
		log.Printf("❌ Panamericana - Error al visitar URL: %v", err)
		errorChan <- fmt.Errorf("error visitando Panamericana: %v", err)
		return
	}

	log.Printf("📊 Panamericana - Resumen: %d elementos procesados, encontrado: %t, precio: $%.0f", elementsFound, found, minPrice)

	if found && minPrice > 0 {
		finalPrice := models.BookPrice{
			Title:     bestTitle,
			Price:     minPrice,
			Source:    "Panamericana",
			ScrapedAt: time.Now(),
		}
		log.Printf("🎉 Panamericana - Precio final: $%.0f para '%s'", minPrice, bestTitle)
		priceChan <- finalPrice
	} else {
		log.Printf("❌ Panamericana - No se encontraron resultados para '%s' (elementos: %d)", bookTitle, elementsFound)
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

// Método processPanamericanaItem eliminado - ya no se usa con el nuevo scraping HTML

// scrapeCasaDelLibro realiza scraping en Casa del Libro Colombia
func (s *Scraper) scrapeCasaDelLibro(bookTitle string, priceChan chan<- models.BookPrice, errorChan chan<- error) {
	defer func() {
		if r := recover(); r != nil {
			log.Printf("❌ PANIC en Casa del Libro scraper: %v", r)
			errorChan <- fmt.Errorf("panic en Casa del Libro scraper: %v", r)
		}
	}()

	log.Printf("🏪 INICIANDO scraping de Casa del Libro para: '%s'", bookTitle)

	// Crear collector con configuración
	c := s.createCollector()

	var minPrice float64 = 0
	var bestTitle string
	found := false
	elementsFound := 0

	// Configurar callback específico para Casa del Libro
	// Basado en la estructura HTML observada en las capturas de pantalla
	// Los productos están en li.x-base-grid_result.x-base-grid_item
	c.OnHTML("li.x-base-grid_result.x-base-grid_item", func(e *colly.HTMLElement) {
		elementsFound++
		log.Printf("🔍 Casa del Libro - Elemento %d encontrado (li.x-base-grid_result)", elementsFound)

		// Extraer título del libro desde el enlace con clase x-result-link x-result__description
		title := strings.TrimSpace(e.ChildText("a.x-result-link.x-result__description"))
		
		// Si no encontramos el título, intentar con selectores alternativos
		if title == "" {
			title = strings.TrimSpace(e.ChildText("a.x-result-link"))
			if title == "" {
				title = strings.TrimSpace(e.ChildText("a[data-v-4d589f60]"))
				if title == "" {
					title = strings.TrimSpace(e.ChildText("a"))
				}
			}
		}

		// Extraer precio desde elemento con clase x-currency
		priceText := strings.TrimSpace(e.ChildText("span.x-currency"))

		// Si no encontramos precio en x-currency, intentar otros selectores
		if priceText == "" {
			priceText = strings.TrimSpace(e.ChildText(".x-result-current-price"))
			if priceText == "" {
				// Buscar cualquier span que contenga $ y números
				e.ForEach("span", func(i int, span *colly.HTMLElement) {
					text := strings.TrimSpace(span.Text)
					if strings.Contains(text, "$") && (strings.Contains(text, "000") || strings.Contains(text, ".")) {
						priceText = text
						log.Printf("💰 Casa del Libro - Precio encontrado en span: '%s'", text)
					}
				})
			}
		}

		log.Printf("📚 Casa del Libro encontró libro - Título: '%s', Precio: '%s'", title, priceText)

		if title != "" && priceText != "" && s.matchesTitle(title, bookTitle) {
			if price := s.parsePrice(priceText); price > 0 {
				if minPrice == 0 || price < minPrice {
					minPrice = price
					bestTitle = title
					found = true
					log.Printf("✅ Casa del Libro - Mejor precio: $%.0f - %s", price, title)
				}
			}
		} else if title == "" {
			log.Printf("⚠️ Casa del Libro - Elemento sin título detectado")
		} else if priceText == "" {
			log.Printf("⚠️ Casa del Libro - Elemento sin precio: '%s'", title)
		}
	})

	// También intentar capturar con selectores más amplios basados en la estructura observada
	c.OnHTML("li[data-v-070eaadb], article", func(e *colly.HTMLElement) {
		elementsFound++
		log.Printf("🔍 Casa del Libro - Elemento %d encontrado (estructura alternativa)", elementsFound)

		// Extraer título desde varios posibles selectores
		title := strings.TrimSpace(e.ChildText("a.x-result-link"))
		if title == "" {
			title = strings.TrimSpace(e.ChildText("a[href*='libro']"))
			if title == "" {
				title = strings.TrimSpace(e.ChildText("a"))
			}
		}

		// Extraer precio
		priceText := strings.TrimSpace(e.ChildText("span.x-currency"))
		if priceText == "" {
			// Buscar cualquier elemento que contenga precio
			e.ForEach("span, div", func(i int, elem *colly.HTMLElement) {
				text := strings.TrimSpace(elem.Text)
				if strings.Contains(text, "$") && len(text) < 20 && (strings.Contains(text, "000") || strings.Contains(text, ".")) {
					priceText = text
					log.Printf("💰 Casa del Libro - Precio en elemento: '%s'", text)
				}
			})
		}

		log.Printf("📚 Casa del Libro (alternativo) encontró - Título: '%s', Precio: '%s'", title, priceText)

		if title != "" && priceText != "" && s.matchesTitle(title, bookTitle) {
			if price := s.parsePrice(priceText); price > 0 {
				if minPrice == 0 || price < minPrice {
					minPrice = price
					bestTitle = title
					found = true
					log.Printf("✅ Casa del Libro (alternativo) - Mejor precio: $%.0f - %s", price, title)
				}
			}
		}
	})

	// URL de búsqueda en Casa del Libro basada en el formato observado en las capturas
	// La URL de búsqueda parece ser /search con el parámetro q
	encodedTitle := url.QueryEscape(bookTitle)
	searchURL := fmt.Sprintf("%s/search?q=%s", s.config.CasaDelLibroBaseURL, encodedTitle)

	log.Printf("🔗 Buscando en Casa del Libro: %s", searchURL)

	// Agregar callback para debug y captura de estructura general
	c.OnHTML("html", func(e *colly.HTMLElement) {
		pageTitle := e.ChildText("title")
		log.Printf("📱 Casa del Libro página procesada, título: %s", pageTitle)

		// Contar elementos de diferentes tipos para debug
		articleCount := 0
		liCount := 0
		
		e.ForEach("article", func(i int, article *colly.HTMLElement) {
			articleCount++
		})
		
		e.ForEach("li.x-base-grid_result", func(i int, li *colly.HTMLElement) {
			liCount++
		})
		
		e.ForEach("ul[data-v-070eaadb] li", func(i int, li *colly.HTMLElement) {
			log.Printf("🔍 Casa del Libro - Li del grid %d encontrado", i+1)
			
			// Debug específico para este li
			link := li.DOM.Find("a").First()
			if link.Length() > 0 {
				href, _ := link.Attr("href")
				text := strings.TrimSpace(link.Text())
				log.Printf("  🔗 Link: href='%s', text='%s'", href, text)
			}
			
			// Debug del precio
			li.ForEach("span", func(j int, span *colly.HTMLElement) {
				spanText := strings.TrimSpace(span.Text)
				if strings.Contains(spanText, "$") {
					log.Printf("  💰 Precio encontrado: '%s'", spanText)
				}
			})
		})
		
		log.Printf("📊 Casa del Libro - Conteo elementos: %d articles, %d li.x-base-grid_result", articleCount, liCount)
	})

	// Callback para respuesta exitosa
	c.OnResponse(func(r *colly.Response) {
		log.Printf("✅ Casa del Libro - Respuesta HTTP %d recibida, tamaño: %d bytes", r.StatusCode, len(r.Body))
	})

	// Callback para errores HTTP
	c.OnError(func(r *colly.Response, err error) {
		log.Printf("❌ Error HTTP en Casa del Libro (status %d): %v", r.StatusCode, err)
	})

	log.Printf("🌐 Casa del Libro - Visitando URL: %s", searchURL)

	if err := c.Visit(searchURL); err != nil {
		log.Printf("❌ Casa del Libro - Error al visitar URL: %v", err)
		errorChan <- fmt.Errorf("error visitando Casa del Libro: %v", err)
		return
	}

	log.Printf("📊 Casa del Libro - Resumen: %d elementos procesados, encontrado: %t, precio: $%.0f", elementsFound, found, minPrice)

	if found && minPrice > 0 {
		finalPrice := models.BookPrice{
			Title:     bestTitle,
			Price:     minPrice,
			Source:    "Casa del Libro",
			ScrapedAt: time.Now(),
		}
		log.Printf("🎉 Casa del Libro - Precio final: $%.0f para '%s'", minPrice, bestTitle)
		priceChan <- finalPrice
	} else {
		log.Printf("❌ Casa del Libro - No se encontraron resultados para '%s' (elementos: %d)", bookTitle, elementsFound)
		errorChan <- fmt.Errorf("no se encontró '%s' en Casa del Libro", bookTitle)
	}
}
