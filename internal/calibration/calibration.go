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

// Factors 存储每个 ETF 的静态校准比例 (Official IOPV / File IOPV)
type Factors struct {
	Ratios   map[string]float64 // ETF代码 -> 乘数比例
	Loaded   bool               // 是否成功加载了校准文件
	FileUsed string             // 实际使用的文件路径（用于日志）
}

// LoadMorningDiff 读取盘前差异文件，解析出每个 ETF 的比例系数
func LoadMorningDiff(filesDir string) *Factors {
	factors := &Factors{
		Ratios: make(map[string]float64),
		Loaded: false,
	}

	today := time.Now().Format("20060102")
	filename := fmt.Sprintf("etf_%s_morning_diff.txt", today)
	fullPath := filepath.Join(filesDir, filename)

	file, err := os.Open(fullPath)
	if err != nil {
		if os.IsNotExist(err) {
			log.Printf("📋 [比例校准] 今日无校准文件 (%s)，以原始精度运行", filename)
		} else {
			log.Printf("⚠️ [比例校准] 读取校准文件失败: %v，以原始精度运行", err)
		}
		return factors
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNum := 0
	parsed := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		if lineNum <= 2 || line == "" {
			continue
		}

		// 解析数据行: ETF_ID  File_IOPV  Official_IOPV  Abs_Diff  Pct_Diff
		fields := strings.Fields(line)
		if len(fields) < 3 {
			continue
		}

		etfCode := fields[0]

		fileIOPV, err := strconv.ParseFloat(fields[1], 64)
		if err != nil || fileIOPV <= 0 {
			continue
		}

		officialIOPV, err := strconv.ParseFloat(fields[2], 64)
		if err != nil {
			continue
		}

		// 按用户最新要求：使用比例作为固定值加权
		ratio := officialIOPV / fileIOPV
		factors.Ratios[etfCode] = ratio
		parsed++
	}

	if err := scanner.Err(); err != nil {
		log.Printf("⚠️ [比例校准] 文件读取过程中出错: %v", err)
		return factors
	}

	factors.Loaded = true
	factors.FileUsed = fullPath
	log.Printf("✅ [比例校准] 成功加载 %d 个 ETF 的盘前校准比例", parsed)

	printTopRatios(factors)
	return factors
}

func (f *Factors) GetRatio(etfCode string) float64 {
	if f == nil || f.Ratios == nil {
		return 1.0
	}
	if ratio, exists := f.Ratios[etfCode]; exists {
		return ratio
	}
	return 1.0 // 默认不进行缩放
}

func printTopRatios(factors *Factors) {
	if len(factors.Ratios) == 0 {
		return
	}

	type entry struct {
		code  string
		ratio float64
		diff  float64 // 偏离 1.0 的程度
	}

	var list []entry
	for code, ratio := range factors.Ratios {
		diff := ratio - 1.0
		if diff < 0 {
			diff = -diff
		}
		list = append(list, entry{code: code, ratio: ratio, diff: diff})
	}

	// 找出偏离 1.0 最多的 5 个
	for i := 0; i < len(list) && i < 5; i++ {
		for j := i + 1; j < len(list); j++ {
			if list[j].diff > list[i].diff {
				list[i], list[j] = list[j], list[i]
			}
		}
	}

	top := 5
	if len(list) < top {
		top = len(list)
	}

	log.Println("📊 [比例校准] 乘数偏离最大 TOP-5：")
	for i := 0; i < top; i++ {
		log.Printf("   -> %s : 比例系数 %.6f", list[i].code, list[i].ratio)
	}
}
