-- Script para crear la tabla de libros únicos
-- Esta tabla almacena cada libro una sola vez con su precio más bajo encontrado

USE book_prices;

CREATE TABLE IF NOT EXISTS unique_books (
    id INT AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(255) NOT NULL,
    min_price DECIMAL(10,2) NOT NULL,
    source VARCHAR(100) NOT NULL,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    
    -- Índices para mejorar rendimiento
    INDEX idx_title (title),
    INDEX idx_min_price (min_price),
    INDEX idx_updated_at (updated_at),
    
    -- Restricción única para evitar duplicados por título
    UNIQUE KEY unique_title (title)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;

-- Comentarios de la tabla
ALTER TABLE unique_books COMMENT = 'Tabla que almacena libros únicos con su menor precio encontrado';