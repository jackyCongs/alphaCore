package engine

import (
	"alphacore/internal/models"
)

// Worker 是一个独立的计算单元，管理分配给自己的那部分指数
type Worker struct {
	ID         int
	TickChan   chan []models.Tick       // 接收实时行情的通道
	ResultChan chan<- models.IndexResult // 发送计算结果的通道

	// 内存数据库 (完全无锁)
	MyIndices     map[string]*models.IndexConfig // 该 worker 负责的指数
	StockToIndices map[string][]string           // 股票代码 -> 影响的指数列表映射
	PriceCache    map[string]float64             // 股票最新价缓存
}

func NewWorker(id int, resultChan chan<- models.IndexResult) *Worker {
	return &Worker{
		ID:             id,
		TickChan:       make(chan []models.Tick, 1024), // 缓冲通道，防止阻塞
		ResultChan:     resultChan,
		MyIndices:      make(map[string]*models.IndexConfig),
		StockToIndices: make(map[string][]string),
		PriceCache:     make(map[string]float64),
	}
}

// Start 启动 Worker 的生命周期，绑定在固定的 goroutine 中运行
func (w *Worker) Start() {
	for batch := range w.TickChan {
		affectedIndices := make(map[string]bool)
		var latestTime int64

		// 1. 更新价格，并找出哪些指数受到了影响
		for _, tick := range batch {
			w.PriceCache[tick.Code] = tick.Price
			if tick.Time > latestTime {
				latestTime = tick.Time
			}

			if indices, ok := w.StockToIndices[tick.Code]; ok {
				for _, idxCode := range indices {
					affectedIndices[idxCode] = true
				}
			}
		}

		// 2. 只重新计算受到影响的指数 (微秒级)
		for idxCode := range affectedIndices {
			w.calculateAndPublish(idxCode, latestTime)
		}
	}
}

func (w *Worker) calculateAndPublish(indexCode string, timestamp int64) {
	config := w.MyIndices[indexCode]
	var totalMarketCap float64 = 0.0

	// 遍历该指数的所有成分股，计算：最新价 * 虚拟股数
	for stockCode, syntheticShares := range config.Components {
		price, exists := w.PriceCache[stockCode]
		if !exists || price <= 0 {
			continue // 如果毫无价格记录，暂时跳过（通常开盘快照会补齐）
		}
		totalMarketCap += price * syntheticShares
	}

	// 套用终极公式计算点位
	if config.Divisor <= 0 {
		return
	}
	realtimePoint := totalMarketCap / config.Divisor
	changePct := (realtimePoint - config.PreClose) / config.PreClose

	// 将结果推向汇聚通道
	w.ResultChan <- models.IndexResult{
		IndexCode: indexCode,
		Point:     realtimePoint,
		ChangePct: changePct,
		Time:      timestamp,
	}
}