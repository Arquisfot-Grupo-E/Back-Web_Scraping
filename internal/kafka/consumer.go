package kafka

import (
	"encoding/json"
	"log"
	"time"

	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/db"
	"github.com/Arquisfot-Grupo-E/Back-Web_Scraping/internal/models"
	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type KafkaConsumer struct {
	consumer *kafka.Consumer
	topic    string
	db       *db.Database
}

func NewKafkaConsumer(broker, topic, groupID string, database *db.Database) (*KafkaConsumer, error) {
	c, err := kafka.NewConsumer(&kafka.ConfigMap{
		"bootstrap.servers": broker,
		"group.id":          groupID,
		"auto.offset.reset": "latest", // Empezar desde los mensajes más recientes
	})
	if err != nil {
		return nil, err
	}

	kc := &KafkaConsumer{
		consumer: c,
		topic:    topic,
		db:       database,
	}

	return kc, nil
}

func (kc *KafkaConsumer) Start() error {
	log.Printf("🎧 Iniciando consumer de Kafka para topic: %s", kc.topic)

	// Suscribirse al topic
	err := kc.consumer.SubscribeTopics([]string{kc.topic}, nil)
	if err != nil {
		return err
	}

	log.Printf("✅ Consumer suscrito exitosamente al topic: %s", kc.topic)

	// Loop principal para procesar mensajes
	for {
		// Leer mensaje con timeout de 100ms
		msg, err := kc.consumer.ReadMessage(100 * time.Millisecond)
		if err != nil {
			// Si es timeout, continuar (esto es normal)
			if err.(kafka.Error).Code() == kafka.ErrTimedOut {
				continue
			}
			log.Printf("❌ Error leyendo mensaje de Kafka: %v", err)
			continue
		}

		// Procesar el mensaje
		kc.processMessage(msg)
	}
}

func (kc *KafkaConsumer) processMessage(msg *kafka.Message) {
	log.Printf("📥 Mensaje recibido de Kafka - Topic: %s, Partition: %d, Offset: %d",
		*msg.TopicPartition.Topic, msg.TopicPartition.Partition, msg.TopicPartition.Offset)

	// Deserializar el evento de Kafka
	var kafkaEvent models.KafkaEvent
	if err := json.Unmarshal(msg.Value, &kafkaEvent); err != nil {
		log.Printf("❌ Error deserializando mensaje de Kafka: %v", err)
		return
	}

	log.Printf("📚 Procesando evento - Libro: '%s', Precio: $%.0f, Fuente: %s",
		kafkaEvent.BookTitle, kafkaEvent.MinPrice, kafkaEvent.Source)

	// Solo procesar eventos de scraping con precio válido
	if kafkaEvent.Action != "scraped" || kafkaEvent.MinPrice <= 0 {
		log.Printf("⚠️ Evento ignorado - Acción: %s, Precio: %.0f", kafkaEvent.Action, kafkaEvent.MinPrice)
		return
	}

	// Guardar o actualizar el libro único en la base de datos
	if err := kc.db.UpsertUniqueBook(&kafkaEvent); err != nil {
		log.Printf("❌ Error guardando libro único: %v", err)
		return
	}

	log.Printf("✅ Libro único guardado/actualizado exitosamente: '%s' con precio $%.0f",
		kafkaEvent.BookTitle, kafkaEvent.MinPrice)
}

func (kc *KafkaConsumer) Close() {
	log.Println("🔒 Cerrando consumer de Kafka...")
	kc.consumer.Close()
}