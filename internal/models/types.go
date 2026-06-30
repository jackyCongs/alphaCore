package models

// Tick represents the lightweight market tick data sent from the ingestion pipeline
type Tick struct {
	Code  string  `json:"c"`
	Price float64 `json:"p"`
	Vol   int64   `json:"v"`
	Amt   float64 `json:"a"`
	Time  int64   `json:"t"`
}

// IndexConfig represents the pre-market ETF index configuration
type IndexConfig struct {
	BasketPreClose         float64            `json:"basket_pre_close"`
	EstimatedCash          float64            `json:"estimated_cash"`
	NetAssetValue          float64            `json:"net_asset_value"`
	OriginBasketAmount     float64            `json:"origin_basket_amount"`
	HiddenSubstituteAmount float64          `json:"hidden_substitute_amount"`
	Components             map[string]int64 `json:"components"`
}

type IndexResult struct {
	IndexCode string  `json:"i"`    // ETF Code
	IOPV      float64 `json:"iopv"` // Real-time IOPV
	Rate      float64 `json:"r"`    // Percentage change (decimal format)
	Time      int64   `json:"t"`    // Timestamp
}

type AppConfig struct {
	QmtFilesDir string `json:"qmt_files_dir"`
	MqttBroker  string `json:"mqtt_broker"`
}

type CalcResult struct {
	Code string  `json:"i"`    // ETF Code
	IOPV float64 `json:"iopv"` // Real-time IOPV
	Time int64   `json:"t"`    // Timestamp
}
