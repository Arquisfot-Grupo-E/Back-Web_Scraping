package kafka

import (
	"encoding/json"
	"fmt"
	"log"

	"github.com/confluentinc/confluent-kafka-go/v2/kafka"
)

type KafkaProducer struct {
	producer *kafka.Producer
	topic    string
}

func NewKafkaProducer(broker, topic string) (*KafkaProducer, error) {
	p, err := kafka.NewProducer(&kafka.ConfigMap{
		"bootstrap.servers": broker,
	})
	if err != nil {
		return nil, fmt.Errorf("error creando producer: %v", err)
	}

	kp := &KafkaProducer{
		producer: p,
		topic:    topic,
	}

	// Inicia un goroutine para escuchar los reportes de entrega
	go func() {
		for e := range p.Events() {
			switch ev := e.(type) {
			case *kafka.Message:
				if ev.TopicPartition.Error != nil {
					log.Printf("⚠️ Error al entregar mensaje a Kafka: %v", ev.TopicPartition.Error)
				} else {
					log.Printf("✅ Mensaje entregado correctamente a %v [%d] en offset %v",
						*ev.TopicPartition.Topic, ev.TopicPartition.Partition, ev.TopicPartition.Offset)
				}
			}
		}
	}()

	return kp, nil
}

func (kp *KafkaProducer) Publish(message interface{}) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("error serializando mensaje: %v", err)
	}

	err = kp.producer.Produce(&kafka.Message{
		TopicPartition: kafka.TopicPartition{
			Topic:     &kp.topic,
			Partition: kafka.PartitionAny,
		},
		Value: data,
	}, nil)

	if err != nil {
		return fmt.Errorf("error publicando mensaje en Kafka: %v", err)
	}

	// Espera mínima para asegurar entrega (opcional)
	kp.producer.Flush(500)
	return nil
}

func (kp *KafkaProducer) Close() {
	kp.producer.Close()
}
