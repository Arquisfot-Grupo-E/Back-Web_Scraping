package db

import (
	"database/sql"
	"fmt"
	"log"
	"time"

	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/config"
	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/models"
	_ "github.com/go-sql-driver/mysql" // Driver MySQL
)

type Database struct {
	db *sql.DB
}

// NewDatabase crea una nueva conexión a MySQL
func NewDatabase(cfg *config.Config) (*Database, error) {
	// Abrir conexión con la cadena de conexión del config
	db, err := sql.Open("mysql", cfg.GetDBConnectionString())
	if err != nil {
		return nil, fmt.Errorf("error opening database: %v", err)
	}

	// Configurar pool de conexiones para optimizar rendimiento
	db.SetMaxOpenConns(25)                 // Máximo 25 conexiones abiertas
	db.SetMaxIdleConns(5)                  // Máximo 5 conexiones idle
	db.SetConnMaxLifetime(5 * time.Minute) // Vida máxima de conexión: 5 min

	// Verificar que la conexión funcione
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error pinging database: %v", err)
	}

	log.Println("✅ Conexión a MySQL establecida exitosamente")
	return &Database{db: db}, nil
}

// Close cierra la conexión a la base de datos
func (d *Database) Close() error {
	return d.db.Close()
}

// InsertBookPrice inserta un nuevo precio de libro en la base de datos
func (d *Database) InsertBookPrice(bookPrice *models.BookPrice) error {
	query := `
        INSERT INTO book_prices (title, price, source, scraped_at) 
        VALUES (?, ?, ?, ?)`

	_, err := d.db.Exec(query, bookPrice.Title, bookPrice.Price, bookPrice.Source, bookPrice.ScrapedAt)
	if err != nil {
		return fmt.Errorf("error inserting book price: %v", err)
	}

	return nil
}

// GetAllBooks obtiene todos los libros guardados en la base de datos
// Ordenados por fecha de scraping (más recientes primero)
func (d *Database) GetAllBooks() ([]models.BookPrice, error) {
	query := `
        SELECT id, title, price, source, scraped_at 
        FROM book_prices 
        ORDER BY scraped_at DESC`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying books: %v", err)
	}
	defer rows.Close()

	var books []models.BookPrice
	for rows.Next() {
		var book models.BookPrice
		err := rows.Scan(&book.ID, &book.Title, &book.Price, &book.Source, &book.ScrapedAt)
		if err != nil {
			return nil, fmt.Errorf("error scanning book: %v", err)
		}
		books = append(books, book)
	}

	return books, nil
}

// GetBookPricesByTitle obtiene todos los precios de un libro específico
// Útil para comparar precios del mismo libro en diferentes fuentes
func (d *Database) GetBookPricesByTitle(title string) ([]models.BookPrice, error) {
	query := `
        SELECT id, title, price, source, scraped_at 
        FROM book_prices 
        WHERE title LIKE ? 
        ORDER BY price ASC, scraped_at DESC`

	rows, err := d.db.Query(query, "%"+title+"%")
	if err != nil {
		return nil, fmt.Errorf("error querying book prices by title: %v", err)
	}
	defer rows.Close()

	var books []models.BookPrice
	for rows.Next() {
		var book models.BookPrice
		err := rows.Scan(&book.ID, &book.Title, &book.Price, &book.Source, &book.ScrapedAt)
		if err != nil {
			return nil, fmt.Errorf("error scanning book price: %v", err)
		}
		books = append(books, book)
	}

	return books, nil
}

// Ping verifica si la conexión a la base de datos está activa
// Usado para health checks del servicio
func (d *Database) Ping() error {
	return d.db.Ping()
}

// UpsertUniqueBook inserta o actualiza un libro único con su menor precio
func (d *Database) UpsertUniqueBook(kafkaEvent *models.KafkaEvent) error {
	// Primero, verificar si el libro ya existe
	var existingPrice float64
	var existingID int
	checkQuery := `SELECT id, min_price FROM unique_books WHERE title = ?`
	
	err := d.db.QueryRow(checkQuery, kafkaEvent.BookTitle).Scan(&existingID, &existingPrice)
	
	if err == sql.ErrNoRows {
		// El libro no existe, insertar uno nuevo
		insertQuery := `
			INSERT INTO unique_books (title, min_price, source, updated_at) 
			VALUES (?, ?, ?, ?)`
		
		_, err = d.db.Exec(insertQuery, 
			kafkaEvent.BookTitle, 
			kafkaEvent.MinPrice, 
			kafkaEvent.Source, 
			time.Now())
		
		if err != nil {
			return fmt.Errorf("error inserting unique book: %v", err)
		}
		
		log.Printf("📚 Nuevo libro único insertado: '%s' con precio $%.0f", 
			kafkaEvent.BookTitle, kafkaEvent.MinPrice)
		return nil
	} else if err != nil {
		return fmt.Errorf("error checking existing book: %v", err)
	}
	
	// El libro existe, verificar si necesita actualización
	if kafkaEvent.MinPrice < existingPrice {
		updateQuery := `
			UPDATE unique_books 
			SET min_price = ?, source = ?, updated_at = ? 
			WHERE id = ?`
		
		_, err = d.db.Exec(updateQuery, 
			kafkaEvent.MinPrice, 
			kafkaEvent.Source, 
			time.Now(), 
			existingID)
		
		if err != nil {
			return fmt.Errorf("error updating unique book: %v", err)
		}
		
		log.Printf("📈 Libro único actualizado: '%s' - precio anterior: $%.0f, nuevo precio: $%.0f", 
			kafkaEvent.BookTitle, existingPrice, kafkaEvent.MinPrice)
	} else {
		log.Printf("💰 Libro único no actualizado: '%s' - precio actual $%.0f es menor que el nuevo $%.0f", 
			kafkaEvent.BookTitle, existingPrice, kafkaEvent.MinPrice)
	}
	
	return nil
}

// GetAllUniqueBooks obtiene todos los libros únicos con sus menores precios
func (d *Database) GetAllUniqueBooks() ([]models.UniqueBook, error) {
	query := `
		SELECT id, title, min_price, source, updated_at 
		FROM unique_books 
		ORDER BY updated_at DESC`

	rows, err := d.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error querying unique books: %v", err)
	}
	defer rows.Close()

	var books []models.UniqueBook
	for rows.Next() {
		var book models.UniqueBook
		err := rows.Scan(&book.ID, &book.Title, &book.MinPrice, &book.Source, &book.UpdatedAt)
		if err != nil {
			return nil, fmt.Errorf("error scanning unique book: %v", err)
		}
		books = append(books, book)
	}

	return books, nil
}
