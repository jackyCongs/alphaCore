package engine

import (
	"log"
	"runtime"

	"alphacore/internal/models"
)

type Dispatcher struct {
	Workers    []*Worker
	ResultChan chan models.IndexResult
}

// NewDispatcher 根据 i5-14600K 的核心数动态创建无锁协程池
func NewDispatcher(configMap map[string]models.IndexConfig) *Dispatcher {
	// 榨干 CPU：默认使用全部逻辑线程
	numWorkers := runtime.GOMAXPROCS(0)
	resultChan := make(chan models.IndexResult, 10000)

	dispatcher := &Dispatcher{
		Workers:    make([]*Worker, numWorkers),
		ResultChan: resultChan,
	}

	for i := 0; i < numWorkers; i++ {
		dispatcher.Workers[i] = NewWorker(i, resultChan)
	}

	// 将全市场指数均匀“分片(Shard)”给各个 Worker
	i := 0
	for idxCode, conf := range configMap {
		worker := dispatcher.Workers[i%numWorkers]

		// 深拷贝配置以防共享内存泄漏
		confCopy := conf
		worker.MyIndices[idxCode] = &confCopy

		// 建立快速倒排索引
		for stockCode := range confCopy.Components {
			worker.StockToIndices[stockCode] = append(worker.StockToIndices[stockCode], idxCode)
		}
		i++
	}

	log.Printf("🚀 引擎初始化完成: 挂载 %d 个指数，已开启 %d 个无锁并行计算单元！", len(configMap), numWorkers)
	return dispatcher
}

func (d *Dispatcher) Start() {
	for _, w := range d.Workers {
		go w.Start()
	}
}

// DispatchTicks 极速广播：将 NanoMQ 收到的批次瞬间投递给所有 Worker
func (d *Dispatcher) DispatchTicks(batch []models.Tick) {
	for _, w := range d.Workers {
		// 非阻塞投递：利用缓冲 Channel
		select {
		case w.TickChan <- batch:
		default:
			log.Printf("⚠️ 警告：Worker %d 通道已满，可能发生行情延迟！", w.ID)
		}
	}
}