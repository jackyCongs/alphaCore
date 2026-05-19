package main

import (
	"alphacore/engine"
)

func main() {
	// 1. 初始化依赖数据 (第一阶段 mock 数据)
	engine.RegisterIndex("000300.SH", engine.IndexConfig{
		Name:    "沪深300 (Mock)",
		Divisor: 39802145.0,
		Constituents: []engine.IndexConstituent{
			{Code: "600519.SH", Factor: 1000.0},
			{Code: "601318.SH", Factor: 5000.0},
		},
	})

	// 2. 启动引擎核心网络服务
	engine.StartUDPServer("127.0.0.1:9999")
}
