package engine

import (
	"log"
	"runtime"
	"sort"

	"alphacore/internal/models"
)

type Dispatcher struct {
	Workers    []*Worker
	ResultChan chan models.IndexResult
}

func NewDispatcher(configMap map[string]models.IndexConfig) *Dispatcher {
	// 1. 限制 Go 最大吃 16 线程，腾出 4 线程给 Python/NanoMQ
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

	// 按成分股总体积（算力消耗）进行贪心分配
	type IndexItem struct {
		Code   string
		Config models.IndexConfig
		Size   int // 成分股数量
	}

	var indexList []IndexItem
	for k, v := range configMap {
		indexList = append(indexList, IndexItem{Code: k, Config: v, Size: len(v.Components)})
	}

	// 将所有指数按照成分股数量从大到小严格排序
	sort.Slice(indexList, func(i, j int) bool {
		return indexList[i].Size > indexList[j].Size
	})

	// 动态跟踪记录这 16 个 Worker 目前各自承载的【成分股总数】
	workerLoads := make([]int, numWorkers)

	// 贪心分配：谁的手活最少，就把下一个指数发给谁
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

		// 建立倒排索引
		for stockCode := range confCopy.Components {
			targetWorker.StockToIndices[stockCode] = append(targetWorker.StockToIndices[stockCode], item.Code)
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
