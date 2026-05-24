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
	BasketPreClose float64            `json:"basket_pre_close"`
	EstimatedCash  float64            `json:"estimated_cash"`
	NetAssetValue  float64            `json:"net_asset_value"`
	Components     map[string]float64 `json:"components"`
}

type IndexResult struct {
	IndexCode string  `json:"i"`    // ETF代码
	IOPV      float64 `json:"iopv"` // 实时净值
	Rate      float64 `json:"r"`    // 涨跌幅（纯小数格式）
	Time      int64   `json:"t"`    // 时间戳
}

type AppConfig struct {
	QmtFilesDir string `json:"qmt_files_dir"`
	MqttBroker  string `json:"mqtt_broker"`
}

type CalcResult struct {
	Code string  `json:"i"`    // ETF代码
	IOPV float64 `json:"iopv"` // 实时净值
	Time int64   `json:"t"`    // 时间戳
}
