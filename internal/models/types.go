package models

// Tick 对应 Python 发过来的微型盘口数据
type Tick struct {
	Code  string  `json:"c"`
	Price float64 `json:"p"`
	Vol   int64   `json:"v"`
	Amt   float64 `json:"a"`
	Time  int64   `json:"t"`
}

// IndexConfig 对应盘前 Python 生成的指数配置
type IndexConfig struct {
	PreClose   float64            `json:"pre_close"`
	Divisor    float64            `json:"divisor"`
	Components map[string]float64 `json:"components"`
}

// IndexResult 对应计算完毕后发回 NanoMQ 的实时指数结果
type IndexResult struct {
	IndexCode string  `json:"i"` // 指数代码
	Point     float64 `json:"p"` // 实时点位
	ChangePct float64 `json:"r"` // 涨跌幅
	Time      int64   `json:"t"` // 时间戳
}