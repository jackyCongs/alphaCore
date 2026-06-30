package engine

import (
	"log"
	"runtime"
	"sort"

	"alphacore/internal/calibration"
	"alphacore/internal/models"
)

type Dispatcher struct {
	Workers    []*Worker
	ResultChan chan models.IndexResult
}

func NewDispatcher(configMap map[string]models.IndexConfig, calFactors *calibration.Factors) *Dispatcher {
	numWorkers := runtime.NumCPU() - 4
	if numWorkers < 4 {
		numWorkers = 4
	}
	runtime.GOMAXPROCS(numWorkers)

	resultChan := make(chan models.IndexResult, 10000)
	dispatcher := &Dispatcher{
		Workers:    make([]*Worker, numWorkers),
		ResultChan: resultChan,
	}

	for i := 0; i < numWorkers; i++ {
		dispatcher.Workers[i] = NewWorker(i, resultChan)
	}

	type IndexItem struct {
		Code   string
		Config models.IndexConfig
		Size   int // Number of components
	}

	var indexList []IndexItem
	for k, v := range configMap {
		indexList = append(indexList, IndexItem{Code: k, Config: v, Size: len(v.Components)})
	}

	sort.Slice(indexList, func(i, j int) bool {
		return indexList[i].Size > indexList[j].Size
	})

	workerLoads := make([]int, numWorkers)

	for _, item := range indexList {
		minIndex := 0
		minLoad := workerLoads[0]
		for wID := 1; wID < numWorkers; wID++ {
			if workerLoads[wID] < minLoad {
				minLoad = workerLoads[wID]
				minIndex = wID
			}
		}

		targetWorker := dispatcher.Workers[minIndex]
		confCopy := item.Config
		targetWorker.MyIndices[item.Code] = &confCopy

		for stockCode := range confCopy.Components {
			targetWorker.StockToIndices[stockCode] = append(targetWorker.StockToIndices[stockCode], item.Code)
		}

		if calFactors != nil {
			if ratio := calFactors.GetRatio(item.Code); ratio != 1.0 && ratio > 0 {
				targetWorker.CalibrationRatios[item.Code] = ratio
			}
		}

		workerLoads[minIndex] += item.Size
	}

	log.Printf("🚀 [AlphaCore Dynamic Load Balancer Initialized]")
	for id, load := range workerLoads {
		log.Printf("   -> Processing Unit Worker_🔥_%02d : Carrying [ %d ] stock component calculation load", id, load)
	}

	return dispatcher
}

func (d *Dispatcher) Start() {
	for _, w := range d.Workers {
		go w.Start()
	}
}

func (d *Dispatcher) DispatchTicks(batch []models.Tick) {
	for _, w := range d.Workers {
		select {
		case w.TickChan <- batch:
		default:
			log.Printf("⚠️ Warning: Worker %d channel is full, market tick processing may be delayed!", w.ID)
		}
	}
}
