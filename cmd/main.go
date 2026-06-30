package main

import (
	"log"
	"os"
	"os/signal"
	"syscall"

	"alphacore/internal/calibration"
	"alphacore/internal/config"
	"alphacore/internal/engine"
	"alphacore/internal/mq"
	"alphacore/internal/web"
)

func main() {
	log.Println("=== [AlphaCore] High-Performance Calculation Engine Starting ===")

	// 1. A clean, single-line call to load the core settings
	appCfg, err := config.LoadAppConfig("./config.json")
	if err != nil {
		log.Fatalf("❌ Failed to load configuration file: %v", err)
	}
	log.Printf("📂 Configuration loaded successfully. Data path: %s, Message broker: %s", appCfg.QmtFilesDir, appCfg.MqttBroker)

	// 2. Load the ETF configurations prepared by the ingestion pipeline
	configMap, err := config.LoadIndexConfig(appCfg.QmtFilesDir)
	if err != nil {
		log.Fatalf("❌ Failed to initialize core ETF configuration: %v", err)
	}

	// 2.5 Load pre-market morning calibration factors (if today's file exists)
	// Expected format: files/etf_YYYYMMDD_morning_diff.txt
	// Apply scaling factors for global calibration
	calFactors := calibration.LoadMorningDiff("./files")

	// 3. Initialize the lock-free concurrent execution engine with calibration factors
	dispatcher := engine.NewDispatcher(configMap, calFactors)
	dispatcher.Start()

	// 3.5 Launch the real-time web dashboard
	stateManager := web.NewStateManager()
	webServer := web.NewServer(stateManager)
	go func() {
		if err := webServer.Run(":8080"); err != nil {
			log.Fatalf("❌ Web server error: %v", err)
		}
	}()

	// 4. Connect to NanoMQ and start the pub/sub message bus
	mqClient := mq.NewNanoMQClient(appCfg.MqttBroker, dispatcher, webServer)
	mqClient.StartResultPublisher()
	mqClient.Subscribe()

	log.Println("🟢 Engine successfully initialized. Waiting for real-time tick stream...")

	// 5. Wait for termination signals gracefully
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	log.Println("🛑 Shutdown signal received. Terminating engine...")
	mqClient.Client.Disconnect(250)
}

