package calibration

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Offsets 存储每个 ETF 的静态校准偏移量（官方IOPV - 我方IOPV）
// 这个值在盘前加载后全天不变，是一个"常量补丁"
type Offsets struct {
	Data    map[string]float64 // ETF代码 -> IOPV偏移量
	Loaded  bool               // 是否成功加载了校准文件
	FileUsed string            // 实际使用的文件路径（用于日志）
}

// LoadMorningDiff 读取盘前差异文件，解析出每个 ETF 的净值偏移量
// 文件命名规则: etf_YYYYMMDD_morning_diff.txt
// 如果当天文件不存在（说明前一天很准确不需要校准），返回空偏移量，不报错
func LoadMorningDiff(filesDir string) *Offsets {
	offsets := &Offsets{
		Data:   make(map[string]float64),
		Loaded: false,
	}

	// 根据当前日期构造文件名
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

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// 跳过标题行（第1行）和分隔线（第2行）
		if lineNum <= 2 {
			continue
		}

		// 跳过空行
		if line == "" {
			continue
		}

		// 解析数据行: ETF_ID  File_IOPV  Official_IOPV  Abs_Diff  Pct_Diff
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}

		etfCode := fields[0]

		// Abs Diff 是第4列（index=3），这是我们需要的校准常量
		// 正值 = 官方比我们算的高（我们漏算了某些成分），需要 +offset
		// 负值 = 官方比我们算的低（我们多算了某些成分），需要 -offset（自动处理，因为值本身就是负的）
		absDiff, err := strconv.ParseFloat(fields[3], 64)
		if err != nil {
			log.Printf("⚠️ [校准] 第%d行解析失败 (ETF=%s): %v", lineNum, etfCode, err)
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
	log.Printf("✅ [校准] 成功加载 %d 个 ETF 的盘前校准偏移量 (文件: %s)", parsed, filename)

	// 打印偏差最大的前5个，帮助观察
	printTopOffsets(offsets)

	return offsets
}

// GetOffset 获取指定 ETF 的校准偏移量，如果不存在则返回 0（即不校准）
func (o *Offsets) GetOffset(etfCode string) float64 {
	if o == nil || o.Data == nil {
		return 0
	}
	return o.Data[etfCode] // map 未命中自动返回 0
}

// printTopOffsets 打印偏差最大的 ETF，方便盘前快速确认
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
		abs := offset
		if abs < 0 {
			abs = -abs
		}
		list = append(list, entry{code: code, offset: offset, absPct: abs})
	}

	// 简单选出前5个最大偏差（冒泡即可，数据量不大）
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

	log.Println("📊 [校准] 偏差最大 TOP-5：")
	for i := 0; i < top; i++ {
		log.Printf("   -> %s : 偏移 %+.6f", list[i].code, list[i].offset)
	}
}
