package engine

import (
	"fmt"
	"sync"
	"time"
)

// calculateSingle 计算单个指数
func calculateSingle(cfg IndexConfig) float64 {
	var totalSum float64 = 0
	for _, c := range cfg.Constituents {
		// 通过安全的 GetPrice 方法获取价格
		if p, ok := GetPrice(c.Code); ok {
			totalSum += p * c.Factor
		}
	}

	if cfg.Divisor == 0 {
		return 0
	}
	return totalSum / cfg.Divisor
}

// RunBatchCalculation 触发全量并发计算
func RunBatchCalculation() {
	start := time.Now()
	var wg sync.WaitGroup

	for _, cfg := range indexRegistry {
		wg.Add(1)
		go func(c IndexConfig) {
			defer wg.Done()
			val := calculateSingle(c)
			// TODO: 后期在这里将 val 通过网络发送回 Python，目前先打印
			fmt.Printf("[%s] 实时指数点位: %.4f\n", c.Name, val)
		}(cfg)
	}

	wg.Wait()
	fmt.Printf(">>> 引擎计算批次完成，耗时: %v\n", time.Since(start))
}
