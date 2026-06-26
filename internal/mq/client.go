package mq

import (
	"encoding/json"
	"log"
	"time"

	"alphacore/internal/engine"
	"alphacore/internal/models"
	"alphacore/internal/web"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type NanoMQClient struct {
	Client     mqtt.Client
	Dispatcher *engine.Dispatcher
	WebServer  *web.Server
}

func NewNanoMQClient(broker string, dispatcher *engine.Dispatcher, webServer *web.Server) *NanoMQClient {
	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID("AlphaCore_Go_Engine")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetCleanSession(true)

	// Set up the default message handler for incoming publications
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		if msg.Topic() == "alphacore/tick/batch" {
			var tickBatch []models.Tick
			// High-performance JSON decoding
			if err := json.Unmarshal(msg.Payload(), &tickBatch); err != nil {
				log.Printf("❌ Failed to decode tick batch: %v", err)
				return
			}
			// Dispatch to calculation engine
			dispatcher.DispatchTicks(tickBatch)
		}
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("❌ Failed to connect to NanoMQ: %v", token.Error())
	}

	log.Printf("✅ Connected to NanoMQ: %s", broker)
	return &NanoMQClient{
		Client:     client,
		Dispatcher: dispatcher,
		WebServer:  webServer,
	}
}

func (m *NanoMQClient) Subscribe() {
	topic := "alphacore/tick/batch"
	if token := m.Client.Subscribe(topic, 0, nil); token.Wait() && token.Error() != nil {
		log.Fatalf("❌ Failed to subscribe to topic %s: %v", topic, token.Error())
	}
	log.Printf("📡 Subscribed to tick feed topic: %s", topic)
}

// StartResultPublisher launches a dedicated routine to batch-publish results back to NanoMQ
func (m *NanoMQClient) StartResultPublisher() {
	go func() {
		// To optimize I/O, we batch publications every 50ms or when the buffer reaches 100 items
		var batch []models.IndexResult
		ticker := time.NewTicker(50 * time.Millisecond)

		for {
			select {
			case result := <-m.Dispatcher.ResultChan:
				batch = append(batch, result)
				if len(batch) >= 100 {
					m.publishBatch(&batch)
				}
			case <-ticker.C:
				if len(batch) > 0 {
					m.publishBatch(&batch)
				}
			}
		}
	}()
}

func (m *NanoMQClient) publishBatch(batch *[]models.IndexResult) {
	payload, err := json.Marshal(*batch)
	if err == nil {
		// Publish to the real-time index topic
		m.Client.Publish("alphacore/index/realtime", 0, false, payload)
	}
	
	// Push update to the web server
	if m.WebServer != nil {
		m.WebServer.PushUpdate(*batch)
	}
	
	// Clear slice while retaining capacity for memory reuse
	*batch = (*batch)[:0]
}
