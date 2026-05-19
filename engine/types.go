package engine

// StockTick 代表从 Python 传来的单只股票快照
type StockTick struct {
	Code  string  `json:"c"`
	Price float64 `json:"p"`
}

// IndexConstituent 指数成分股及其权重参数
type IndexConstituent struct {
	Code   string
	Factor float64
}

// IndexConfig 指数定义
type IndexConfig struct {
	Name         string
	Divisor      float64
	Constituents []IndexConstituent
}
