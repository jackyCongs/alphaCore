package engine

import (
	"log"
	"math"
	"time"

	"alphacore/internal/models"
)

// Worker is an independent calculation unit that manages its assigned subset of ETFs
type Worker struct {
	ID         int
	TickChan   chan []models.Tick        // Channel for receiving real-time ticks
	ResultChan chan<- models.IndexResult // Channel for publishing computed results

	// In-memory database (completely lock-free)
	MyIndices           map[string]*models.IndexConfig // ETFs assigned to this worker
	StockToIndices      map[string][]string            // Stock code -> List of affected ETFs mapping
	PriceCache          map[string]int64               // Latest stock price cache (scaled by 1000)
	CurrentBasketValues map[string]int64               // Real-time incremental basket value (scaled by 1000)

	// Pre-market calibration factors (ETF code -> IOPV scaling ratio)
	// This ratio equals (morning official IOPV / morning calculated IOPV)
	CalibrationRatios map[string]float64
	// ReadyCache indicates whether all component prices for an ETF have been received
	ReadyCache map[string]bool
	StartTime  time.Time
}

func NewWorker(id int, resultChan chan<- models.IndexResult) *Worker {
	return &Worker{
		ID:                  id,
		TickChan:            make(chan []models.Tick, 1024), // Buffered channel to prevent blocking
		ResultChan:          resultChan,
		MyIndices:           make(map[string]*models.IndexConfig),
		StockToIndices:      make(map[string][]string),
		PriceCache:          make(map[string]int64),
		CurrentBasketValues: make(map[string]int64),
		CalibrationRatios:   make(map[string]float64),
		ReadyCache:          make(map[string]bool),
		StartTime:           time.Now(),
	}
}

// Start initiates the worker's lifecycle, pinned to a dedicated goroutine
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
	if !w.ReadyCache[etfCode] {
		missing := false
		for stock := range config.Components {
			if _, ok := w.PriceCache[stock]; !ok {
				if time.Since(w.StartTime) > 10*time.Second {
					log.Printf("⚠️ ETF %s waiting for price of component %s", etfCode, stock)
				}
				missing = true
				break
			}
		}
		if missing {
			return
		}
		w.ReadyCache[etfCode] = true
	}
	realtimeBasketValue := float64(w.CurrentBasketValues[etfCode]) / 1000.0

	yesterdayTotalValue := config.OriginBasketAmount
	if yesterdayTotalValue <= 0 {
		return
	}

	realtimeTotalValue := realtimeBasketValue + config.EstimatedCash + config.HiddenSubstituteAmount
	realtimeIOPV := config.NetAssetValue * (realtimeTotalValue / yesterdayTotalValue)

	// Apply pre-market calibration scaling
	// i.e., Real-time IOPV = Raw Calculated IOPV * Calibration Ratio
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
