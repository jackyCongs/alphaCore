package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"alphacore/internal/config"
	"alphacore/internal/engine"
	"alphacore/internal/mq"
)

func main() {
	log.Println("=== [AlphaCore] 极速核心计算引擎启动 ===")

	// 1. 读取 Python 准备好的 JSON 弹药库
	configMap, err := config.LoadIndexConfig("config.json")
	if err != nil {
		log.Fatalf("❌ 初始化失败: %v", err)
	}

	// 2. 创建无锁并发调度引擎
	dispatcher := engine.NewDispatcher(configMap)
	dispatcher.Start()

	// 3. 连接 NanoMQ 并启动收发总线
	// 如果你的 NanoMQ 在其他机器，请修改为 tcp://IP:1883
	mqClient := mq.NewNanoMQClient("tcp://127.0.0.1:1883", dispatcher)
	mqClient.StartResultPublisher()
	mqClient.Subscribe()

	log.Println("🟢 引擎全面升空！等待开盘数据流注入...")

	// 4. 优雅等待关闭信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("🛑 接收到退出信号，关闭引擎...")
	mqClient.Client.Disconnect(250)
}