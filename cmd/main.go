package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"alphacore/internal/config"
	"alphacore/internal/engine"
	"alphacore/internal/mq"
	"alphacore/internal/web"
)

func main() {
	log.Println("=== [AlphaCore] 极速核心计算引擎启动 ===")

	// 1. A clean, single-line call to load the core settings
	appCfg, err := config.LoadAppConfig("./config.json")
	if err != nil {
		log.Fatalf("❌ 配置文件加载失败: %v", err)
	}
	log.Printf("📂 成功加载配置。数据目录: %s, 消息队列: %s", appCfg.QmtFilesDir, appCfg.MqttBroker)

	// 2. 读取 Python 准备好的 JSON 弹药库
	configMap, err := config.LoadIndexConfig(appCfg.QmtFilesDir)
	if err != nil {
		log.Fatalf("❌ 初始化核心数据失败: %v", err)
	}

	// 3. 创建无锁并发调度引擎
	dispatcher := engine.NewDispatcher(configMap)
	dispatcher.Start()

	// 3.5 启动 Web 实时仪表盘
	stateManager := web.NewStateManager()
	webServer := web.NewServer(stateManager)
	go func() {
		if err := webServer.Run(":8080"); err != nil {
			log.Fatalf("❌ Web 服务异常: %v", err)
		}
	}()

	// 4. 连接 NanoMQ 并启动收发总线
	mqClient := mq.NewNanoMQClient(appCfg.MqttBroker, dispatcher, webServer)
	mqClient.StartResultPublisher()
	mqClient.Subscribe()

	log.Println("🟢 引擎全面升空！等待开盘数据流注入...")

	// 5. 优雅等待关闭信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("🛑 接收到退出信号，关闭引擎...")
	mqClient.Client.Disconnect(250)
}
