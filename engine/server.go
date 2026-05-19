package engine

import (
	"encoding/json"
	"fmt"
	"net"
)

// StartUDPServer 启动网络监听服务
func StartUDPServer(port string) {
	addr, err := net.ResolveUDPAddr("udp", port)
	if err != nil {
		panic(err)
	}

	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	fmt.Printf("🚀 AlphaCore Engine Ready. Listening on %s...\n", port)

	buf := make([]byte, 1024*512)
	for {
		n, _, err := conn.ReadFromUDP(buf)
		if err != nil {
			continue
		}

		var ticks []StockTick
		if err := json.Unmarshal(buf[:n], &ticks); err != nil {
			fmt.Println("❌ 数据反序列化失败:", err)
			continue
		}

		// 1. 更新内存状态
		UpdatePrices(ticks)

		// 2. 异步触发计算，不阻塞下一次网络接收
		go RunBatchCalculation()
	}
}
