package engine

import "sync"

var (
	// package-private: 外部包无法直接访问，保证数据安全
	priceMap   = make(map[string]float64)
	priceMutex sync.RWMutex

	indexRegistry = make(map[string]IndexConfig)
)

// UpdatePrices 批量安全更新价格
func UpdatePrices(ticks []StockTick) {
	priceMutex.Lock()
	defer priceMutex.Unlock()
	for _, t := range ticks {
		priceMap[t.Code] = t.Price
	}
}

// GetPrice 安全获取单只股票价格及其是否存在
func GetPrice(code string) (float64, bool) {
	priceMutex.RLock()
	defer priceMutex.RUnlock()
	p, ok := priceMap[code]
	return p, ok
}

// RegisterIndex 注册一个指数到内存
func RegisterIndex(code string, cfg IndexConfig) {
	indexRegistry[code] = cfg
}
