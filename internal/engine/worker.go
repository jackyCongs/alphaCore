package engine

import (
	"math"

	"alphacore/internal/models"
)

// Worker 是一个独立的计算单元，管理分配给自己的那部分指数
type Worker struct {
	ID         int
	TickChan   chan []models.Tick        // 接收实时行情的通道
	ResultChan chan<- models.IndexResult // 发送计算结果的通道

	// 内存数据库 (完全无锁)
	MyIndices           map[string]*models.IndexConfig // 该 worker 负责的指数
	StockToIndices      map[string][]string            // 股票代码 -> 影响的指数列表映射
	PriceCache          map[string]int64               // 股票最新价缓存 (放大1000倍)
	CurrentBasketValues map[string]int64               // 实时篮子增量价值 (放大1000倍)

	// 盘前校准比例系数 (ETF代码 -> IOPV缩放比例)
	// 这个值等于 (早上官方IOPV / 早上我方IOPV)
	CalibrationRatios map[string]float64
}

func NewWorker(id int, resultChan chan<- models.IndexResult) *Worker {
	return &Worker{
		ID:                id,
		TickChan:          make(chan []models.Tick, 1024), // 缓冲通道，防止阻塞
		ResultChan:        resultChan,
		MyIndices:         make(map[string]*models.IndexConfig),
		StockToIndices:      make(map[string][]string),
		PriceCache:          make(map[string]int64),
		CurrentBasketValues: make(map[string]int64),
		CalibrationRatios:   make(map[string]float64),
	}
}

// Start 启动 Worker 的生命周期，绑定在固定的 goroutine 中运行
func (w *Worker) Start() {
	for batch := range w.TickChan {
		affectedIndices := make(map[string]bool)
		var latestTime int64

		for _, tick := range batch {
			tickPriceInt := int64(math.Round(tick.Price * 1000.0))
			oldPrice := w.PriceCache[tick.Code]
			deltaPrice := tickPriceInt - oldPrice
			w.PriceCache[tick.Code] = tickPriceInt

			if tick.Time > latestTime {
				latestTime = tick.Time
			}

			if indices, ok := w.StockToIndices[tick.Code]; ok {
				for _, idxCode := range indices {
					shares := w.MyIndices[idxCode].Components[tick.Code]
					w.CurrentBasketValues[idxCode] += deltaPrice * shares
					affectedIndices[idxCode] = true
				}
			}
		}
		for idxCode := range affectedIndices {
			w.calculateAndPublish(idxCode, latestTime)
		}
	}
}

func (w *Worker) calculateAndPublish(etfCode string, timestamp int64) {
	config := w.MyIndices[etfCode]
	realtimeBasketValue := float64(w.CurrentBasketValues[etfCode]) / 1000.0

	yesterdayTotalValue := config.OriginBasketAmount
	if yesterdayTotalValue <= 0 {
		return
	}

	realtimeTotalValue := realtimeBasketValue + config.EstimatedCash + config.HiddenSubstituteAmount
	realtimeIOPV := config.NetAssetValue * (realtimeTotalValue / yesterdayTotalValue)

	// 套用盘前校准比例（固定值加权）
	// 即：实时净值 = 原始计算净值 * (早上官方 / 早上我方)
	if ratio, ok := w.CalibrationRatios[etfCode]; ok && ratio > 0 {
		realtimeIOPV *= ratio
	}

	var changePct float64 = 0.0
	if config.NetAssetValue > 0 {
		changePct = (realtimeIOPV / config.NetAssetValue) - 1.0
	}

	w.ResultChan <- models.IndexResult{
		IndexCode: etfCode,
		IOPV:      realtimeIOPV,
		Rate:      changePct,
		Time:      timestamp,
	}
}
