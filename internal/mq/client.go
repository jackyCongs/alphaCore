package mq

import (
	"encoding/json"
	"log"
	"time"

	"alphacore/internal/engine"
	"alphacore/internal/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type NanoMQClient struct {
	Client     mqtt.Client
	Dispatcher *engine.Dispatcher
}

func NewNanoMQClient(broker string, dispatcher *engine.Dispatcher) *NanoMQClient {
	opts := mqtt.NewClientOptions().AddBroker(broker).SetClientID("AlphaCore_Go_Engine")
	opts.SetKeepAlive(60 * time.Second)
	opts.SetCleanSession(true)

	// 设置全局消息到达的回调函数
	opts.SetDefaultPublishHandler(func(client mqtt.Client, msg mqtt.Message) {
		if msg.Topic() == "alphacore/tick/batch" {
			var tickBatch []models.Tick
			// 极速解码 JSON
			if err := json.Unmarshal(msg.Payload(), &tickBatch); err != nil {
				log.Printf("❌ 解析 Tick 数据失败: %v", err)
				return
			}
			// 投递给计算引擎
			dispatcher.DispatchTicks(tickBatch)
		}
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("❌ 连接 NanoMQ 失败: %v", token.Error())
	}

	log.Printf("✅ 成功连接到 NanoMQ: %s", broker)
	return &NanoMQClient{
		Client:     client,
		Dispatcher: dispatcher,
	}
}

func (m *NanoMQClient) Subscribe() {
	topic := "alphacore/tick/batch"
	if token := m.Client.Subscribe(topic, 0, nil); token.Wait() && token.Error() != nil {
		log.Fatalf("❌ 订阅主题 %s 失败: %v", topic, token.Error())
	}
	log.Printf("📡 成功订阅 Tick 洪流: %s", topic)
}

// StartResultPublisher 开启独立协程，将算好的指数结果打包发回 NanoMQ
func (m *NanoMQClient) StartResultPublisher() {
	go func() {
		// 为了压榨 I/O，我们攒够一定数量或者每隔 50ms 批量发一次
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
		// 发布到统一的实时期货主题
		m.Client.Publish("alphacore/index/realtime", 0, false, payload)
	}
	// 清空切片复用内存
	*batch = (*batch)[:0]
}