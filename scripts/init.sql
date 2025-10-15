-- ================================
-- Script de inicialización de base de datos para el microservicio de web scraping
-- Este script crea la base de datos y tabla necesarias para almacenar precios de libros
-- ================================

-- Crear base de datos si no existe
CREATE DATABASE IF NOT EXISTS book_prices 
CHARACTER SET utf8mb4 
COLLATE utf8mb4_unicode_ci;

-- Usar la base de datos
USE book_prices;

-- Crear tabla para almacenar precios de libros de diferentes fuentes
CREATE TABLE IF NOT EXISTS book_prices (
    id INT AUTO_INCREMENT PRIMARY KEY,
    title VARCHAR(500) NOT NULL COMMENT 'Título del libro',
    price DECIMAL(10,2) NOT NULL COMMENT 'Precio del libro',
    source VARCHAR(100) NOT NULL COMMENT 'Fuente del precio (Casa del Libro, Buscalibre, etc.)',
    scraped_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP COMMENT 'Fecha y hora del scraping',
    
    -- Índices para mejorar rendimiento de consultas
    INDEX idx_title (title(255)),      -- Búsquedas por título
    INDEX idx_source (source),         -- Filtros por fuente
    INDEX idx_scraped_at (scraped_at), -- Ordenamiento por fecha
    INDEX idx_price (price)            -- Ordenamiento por precio
) ENGINE=InnoDB COMMENT='Almacena precios de libros obtenidos por web scraping';

-- Datos de prueba (opcional - para testing)
INSERT INTO book_prices (title, price, source, scraped_at) VALUES
('Cien años de soledad', 85000.00, 'Casa del Libro', NOW()),
('Cien años de soledad', 82500.00, 'Buscalibre', NOW()),
('Don Quijote de la Mancha', 65000.00, 'Casa del Libro', NOW()),
('Don Quijote de la Mancha', 68000.00, 'Buscalibre', NOW()),
('El Principito', 25000.00, 'Casa del Libro', NOW()),
('El Principito', 23500.00, 'Buscalibre', NOW())
ON DUPLICATE KEY UPDATE title=title; -- Evita errores si ya existen

-- Verificar la estructura creada
SHOW TABLES;
DESCRIBE book_prices;