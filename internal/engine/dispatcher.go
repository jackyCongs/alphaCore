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

func NewDispatcher(configMap map[string]models.IndexConfig, calOffsets *calibration.Offsets) *Dispatcher {
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
		Size   int // 成分股数量
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

		if calOffsets != nil {
			if offset := calOffsets.GetOffset(item.Code); offset != 0 {
				targetWorker.CalibrationOffsets[item.Code] = offset
			}
		}

		workerLoads[minIndex] += item.Size
	}

	log.Printf("🚀 [AlphaCore 动态负载均衡点火成功]")
	for id, load := range workerLoads {
		log.Printf("   -> 计算单元 Worker_🔥_%02d : 已承载实时成分股乘加压力 [ %d ] 只", id, load)
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
			log.Printf("⚠️ 警告：Worker %d 通道已满，可能发生行情延迟！", w.ID)
		}
	}
}
