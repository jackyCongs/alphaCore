package engine

import (
	"alphacore/internal/models"
	"fmt"
)

// Worker 是一个独立的计算单元，管理分配给自己的那部分指数
type Worker struct {
	ID         int
	TickChan   chan []models.Tick        // 接收实时行情的通道
	ResultChan chan<- models.IndexResult // 发送计算结果的通道

	// 内存数据库 (完全无锁)
	MyIndices      map[string]*models.IndexConfig // 该 worker 负责的指数
	StockToIndices map[string][]string            // 股票代码 -> 影响的指数列表映射
	PriceCache     map[string]float64             // 股票最新价缓存
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

func (w *Worker) calculateAndPublish(etfCode string, timestamp int64) {
	config := w.MyIndices[etfCode]
	var realtimeBasketValue float64 = 0.0

	// 1. 遍历该 ETF 的所有成分股，计算：最新价 * PCF绝对股数
	for stockCode, shares := range config.Components {
		price, exists := w.PriceCache[stockCode]
		if !exists {
			// fmt.Println(fmt.Sprintf("[缺失Tick] %s 从未收到数据！", stockCode))
		} else if price <= 0.01 {
			fmt.Println(fmt.Sprintf("[零价异常] %s 收到了Tick，但价格极低: %f", stockCode, price))
		}
		realtimeBasketValue += price * shares
	}

	// 2. 套用 IOPV 物理守恒公式
	yesterdayTotalValue := config.OriginBasketAmount
	if yesterdayTotalValue <= 0 {
		return // 防止除以 0 导致引擎崩溃
	}

	realtimeTotalValue := realtimeBasketValue + config.EstimatedCash + config.HiddenSubstituteAmount
	// 实时净值 = 昨日净值 * (实时总价值 / 昨日总价值)
	realtimeIOPV := config.NetAssetValue * (realtimeTotalValue / yesterdayTotalValue)

	var changePct float64 = 0.0
	if config.NetAssetValue > 0 {
		changePct = (realtimeIOPV / config.NetAssetValue) - 1.0
	}

	// 3. 将结果推向汇聚通道
	w.ResultChan <- models.IndexResult{
		IndexCode: etfCode,
		IOPV:      realtimeIOPV,
		Rate:      changePct,
		Time:      timestamp,
	}
}
