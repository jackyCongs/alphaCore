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

// Factors stores the static calibration ratio for each ETF (Official IOPV / File IOPV)
type Factors struct {
	Ratios   map[string]float64 // ETF Code -> Scaling Ratio
	Loaded   bool               // Whether the calibration file was loaded successfully
	FileUsed string             // The file path used (for logging)
}

// LoadMorningDiff reads the pre-market morning difference file and extracts the calibration ratio for each ETF
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
			log.Printf("📋 [Calibration] No calibration file for today (%s). Running with raw precision.", filename)
		} else {
			log.Printf("⚠️ [Calibration] Failed to read calibration file: %v. Running with raw precision.", err)
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

		// Parse data row: ETF_ID  File_IOPV  Official_IOPV  Abs_Diff  Pct_Diff
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

		// Apply ratio scaling based on morning deviation
		ratio := officialIOPV / fileIOPV
		factors.Ratios[etfCode] = ratio
		parsed++
	}

	if err := scanner.Err(); err != nil {
		log.Printf("⚠️ [Calibration] Error reading file: %v", err)
		return factors
	}

	factors.Loaded = true
	factors.FileUsed = fullPath
	log.Printf("✅ [Calibration] Successfully loaded pre-market calibration ratios for %d ETFs", parsed)

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
	return 1.0 // Default to no scaling
}

func printTopRatios(factors *Factors) {
	if len(factors.Ratios) == 0 {
		return
	}

	type entry struct {
		code  string
		ratio float64
		diff  float64 // Deviation from 1.0
	}

	var list []entry
	for code, ratio := range factors.Ratios {
		diff := ratio - 1.0
		if diff < 0 {
			diff = -diff
		}
		list = append(list, entry{code: code, ratio: ratio, diff: diff})
	}

	// Find the top 5 largest deviations from 1.0
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

	log.Println("📊 [Calibration] Top 5 largest scaling factor deviations:")
	for i := 0; i < top; i++ {
		log.Printf("   -> %s : Scaling Factor %.6f", list[i].code, list[i].ratio)
	}
}
