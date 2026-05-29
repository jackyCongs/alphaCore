package calibration

import (
	"bufio"
	"fmt"
	"log"
	"math"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Offsets 存储每个 ETF 的静态校准偏移量（官方IOPV - 我方IOPV）
// 这个值在盘前加载后全天不变，是一个"常量补丁"
type Offsets struct {
	Data     map[string]float64 // ETF代码 -> IOPV偏移量
	Loaded   bool               // 是否成功加载了校准文件
	FileUsed string             // 实际使用的文件路径（用于日志）
}

// 灵敏度阈值：如果盘前百分比误差小于此值（例如 0.15%），则视为正常误差或盘前报价噪音（如QDII汇率/T-1价格），不强行校准。
const CalibrationThresholdPct = 0.15

// LoadMorningDiff 读取盘前差异文件，解析出每个 ETF 的净值偏移量
// 文件命名规则: etf_YYYYMMDD_morning_diff.txt
// 如果当天文件不存在，返回空偏移量，不报错
func LoadMorningDiff(filesDir string) *Offsets {
	offsets := &Offsets{
		Data:   make(map[string]float64),
		Loaded: false,
	}

	today := time.Now().Format("20060102")
	filename := fmt.Sprintf("etf_%s_morning_diff.txt", today)
	fullPath := filepath.Join(filesDir, filename)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("📋 [校准] 今日无校准文件 (%s)，以原始精度运行", filename)
		} else {
			log.Printf("⚠️ [校准] 读取校准文件失败: %v，以原始精度运行", err)
		}
		return offsets
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	parsed := 0
	ignored := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// 跳过标题行（第1行）和分隔线（第2行）
		if lineNum <= 2 || line == "" {
			continue
		}

		// 解析数据行: ETF_ID  File_IOPV  Official_IOPV  Abs_Diff  Pct_Diff  %
		fields := strings.Fields(line)
		if len(fields) < 5 {
			continue
		}

		etfCode := fields[0]

		// 解析 Abs Diff (第4列)
		absDiff, err := strconv.ParseFloat(fields[3], 64)
		if err != nil {
			log.Printf("⚠️ [校准] 第%d行解析 Abs Diff 失败 (ETF=%s)", lineNum, etfCode)
			continue
		}

		// 解析 Pct Diff (第5列)
		pctDiffStr := strings.ReplaceAll(fields[4], "%", "") // 去掉可能的 % 号
		pctDiff, err := strconv.ParseFloat(pctDiffStr, 64)
		if err != nil {
			log.Printf("⚠️ [校准] 第%d行解析 Pct Diff 失败 (ETF=%s)", lineNum, etfCode)
			continue
		}

		// 【核心逻辑】：只对误差超过阈值的 ETF 开启强行干预！
		// 很多 QDII 或跨市场 ETF 在 9:25 存在虚假的报价差异（由于汇率或T-1数据导致），
		// 开盘后这些差异会自动消失。如果强行把 9:25 的虚假差异加进去，反而会导致盘中反向恶化。
		if math.Abs(pctDiff) < CalibrationThresholdPct {
			ignored++
			continue
		}

		offsets.Data[etfCode] = absDiff
		parsed++
	}

	if err := scanner.Err(); err != nil {
		log.Printf("⚠️ [校准] 文件读取过程中出错: %v", err)
		return offsets
	}

	offsets.Loaded = true
	offsets.FileUsed = fullPath
	log.Printf("✅ [校准] 加载校准数据完成！应用校准: %d 个，因误差极小(<%.2f%%)自动豁免: %d 个", parsed, CalibrationThresholdPct, ignored)

	printTopOffsets(offsets)
	return offsets
}

func (o *Offsets) GetOffset(etfCode string) float64 {
	if o == nil || o.Data == nil {
		return 0
	}
	return o.Data[etfCode]
}

func printTopOffsets(offsets *Offsets) {
	if len(offsets.Data) == 0 {
		return
	}

	type entry struct {
		code   string
		offset float64
		absPct float64
	}

	var list []entry
	for code, offset := range offsets.Data {
		abs := math.Abs(offset)
		list = append(list, entry{code: code, offset: offset, absPct: abs})
	}

	for i := 0; i < len(list) && i < 5; i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].absPct > list[i].absPct {
				list[i], list[j] = list[j], list[i]
			}
		}
	}

	top := 5
	if len(list) < top {
		top = len(list)
	}

	log.Println("📊 [校准] 需要强行干预的偏差最大 TOP-5：")
	for i := 0; i < top; i++ {
		log.Printf("   -> %s : 偏移 %+.6f", list[i].code, list[i].offset)
	}
}
