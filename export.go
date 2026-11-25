package main

import (
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/wailsapp/wails/v2/pkg/runtime"
	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

const maxExportResults = 500

func recognizedRegionCode(iata string) string {
	code := strings.TrimSpace(iata)
	if len(code) != 3 {
		return ""
	}
	for _, letter := range code {
		if (letter < 'A' || letter > 'Z') && (letter < 'a' || letter > 'z') {
			return ""
		}
	}
	return strings.ToUpper(code)
}

func exportRegionName(code, name string) string {
	name = strings.TrimSpace(name)
	if name == "" || name == "未知地区" || strings.EqualFold(name, "UNKNOWN") {
		return GetChineseName(code)
	}
	return name
}

func recognizedSpeedResults(results []SpeedResult, regions map[string]bool) []SpeedResult {
	known := make([]SpeedResult, 0, min(len(results), maxExportResults))
	for _, result := range results {
		code := recognizedRegionCode(result.IataCode)
		if code == "" || (regions != nil && !regions[code]) {
			continue
		}
		result.IataCode = code
		result.ChineseName = exportRegionName(code, result.ChineseName)
		known = append(known, result)
		if len(known) == maxExportResults {
			break
		}
	}
	return known
}

// ExportResults 保留原有测速结果导出入口。
func (a *App) ExportResults(results []SpeedResult, mode string) (string, error) {
	results = recognizedSpeedResults(results, nil)
	if mode == "cm" {
		return a.exportResults(results, "txt", false, true)
	}
	return a.exportResults(results, "csv", false, true)
}

func (a *App) ExportCurrentResults(format, layout string) (string, error) {
	return a.exportCurrentResults(format, layout, nil)
}

func (a *App) ExportCurrentRegionResults(format, layout string, regionCodes []string) (string, error) {
	regions := make(map[string]bool, len(regionCodes))
	for _, rawCode := range regionCodes {
		if code := recognizedRegionCode(rawCode); code != "" {
			regions[code] = true
		}
	}
	if len(regions) == 0 {
		return "", fmt.Errorf("请至少勾选一个导出地区")
	}
	return a.exportCurrentResults(format, layout, regions)
}

func (a *App) exportCurrentResults(format, layout string, regions map[string]bool) (string, error) {
	if format != "csv" && format != "txt" {
		return "", fmt.Errorf("请选择 CSV 或 TXT 格式")
	}
	if layout != "default" && layout != "compact" {
		return "", fmt.Errorf("请选择默认或精简样式")
	}
	results, measured := a.currentExportResults(regions)
	if regions != nil && len(results) == 0 {
		return "", fmt.Errorf("勾选的地区没有可导出结果")
	}
	return a.exportResults(results, format, layout == "compact", measured)
}

func (a *App) currentExportResults(regions map[string]bool) ([]SpeedResult, bool) {
	a.mu.Lock()
	scanned, speedResults := a.lastScanResults, a.lastSpeedResults
	a.mu.Unlock()
	if slices.ContainsFunc(speedResults, func(result SpeedResult) bool { return recognizedRegionCode(result.IataCode) != "" }) {
		return recognizedSpeedResults(speedResults, regions), true
	}

	sorted := slices.DeleteFunc(slices.Clone(scanned), func(result ScanResult) bool {
		code := recognizedRegionCode(result.IataCode)
		return code == "" || (regions != nil && !regions[code])
	})
	sortResultsByLatency(sorted)
	limit := min(maxExportResults, len(sorted))
	results := make([]SpeedResult, 0, limit)
	for _, result := range sorted[:limit] {
		code := recognizedRegionCode(result.IataCode)
		results = append(results, SpeedResult{
			IP: result.IP, Port: result.Port, IataCode: code,
			ChineseName: exportRegionName(code, result.ChineseName), LatencyMs: result.LatencyMs, TestType: "扫描",
		})
	}
	return results, false
}

func (a *App) exportFileTimestamp(now time.Time) string {
	a.mu.Lock()
	millis := max(now.UnixMilli(), a.lastExportMillis+1)
	a.lastExportMillis = millis
	a.mu.Unlock()

	stamp := time.UnixMilli(millis).In(now.Location())
	return fmt.Sprintf("%s_%03d", stamp.Format("20060102_150405"), stamp.Nanosecond()/int(time.Millisecond))
}

func (a *App) exportResults(results []SpeedResult, format string, compact, measured bool) (string, error) {
	if len(results) == 0 {
		return "", fmt.Errorf("没有识别到地区的结果可导出")
	}
	results = results[:min(len(results), maxExportResults)]

	stamp := a.exportFileTimestamp(time.Now())
	options := runtime.SaveDialogOptions{
		DefaultDirectory: ".",
		DefaultFilename:  "cfs_results_" + stamp + ".csv",
		Title:            "保存结果 (CSV)",
		Filters:          []runtime.FileFilter{{DisplayName: "CSV文件", Pattern: "*.csv"}},
	}
	write := writeCSVResults
	if format == "txt" {
		options.DefaultFilename = "cfs_cm_" + stamp + ".txt"
		options.Title = "保存结果 (TXT)"
		options.Filters = []runtime.FileFilter{{DisplayName: "文本文件", Pattern: "*.txt"}}
		write = writeTXTResults
	}
	savePath, err := runtime.SaveFileDialog(a.ctx, options)
	if err != nil || savePath == "" {
		return "", err
	}

	file, err := os.Create(savePath)
	if err != nil {
		return "", err
	}
	writeErr := write(file, results, compact, measured)
	closeErr := file.Close()
	if writeErr != nil {
		return "", writeErr
	}
	if closeErr != nil {
		return "", closeErr
	}
	return savePath, nil
}

// CSV 使用 GBK 编码，以便 Windows Excel 正确显示中文。
func writeCSVResults(destination io.Writer, results []SpeedResult, compact, measured bool) error {
	writer := csv.NewWriter(transform.NewWriter(destination, simplifiedchinese.GBK.NewEncoder()))
	writer.UseCRLF = true
	headers := []string{"排名", "IP地址", "地区码", "地区"}
	if compact {
		headers = append(headers, "端口")
	} else {
		headers = append(headers, "延迟(ms)", "下载速度(MB/s)", "端口", "测速类型")
	}
	if err := writer.Write(headers); err != nil {
		return err
	}
	for i, result := range results {
		port := strconv.Itoa(result.Port)
		row := []string{strconv.Itoa(i + 1), result.IP, result.IataCode}
		if compact {
			row = append(row, compactRegionLabel(result.IataCode), port)
		} else {
			speed := ""
			if measured {
				speed = fmt.Sprintf("%.2f", result.DownloadSpeed)
			}
			row = append(row, result.ChineseName, fmt.Sprintf("%.2f", result.LatencyMs), speed, port, result.TestType)
		}
		if err := writer.Write(row); err != nil {
			return err
		}
	}
	writer.Flush()
	return writer.Error()
}

// TXT 保留原有 CM 地址与地区表示法，IPv6 地址由方括号包围。
func writeTXTResults(destination io.Writer, results []SpeedResult, compact, measured bool) error {
	for _, result := range results {
		address := net.JoinHostPort(result.IP, strconv.Itoa(result.Port))
		var label string
		if compact {
			label = compactRegionLabel(result.IataCode)
		} else {
			code, name := result.IataCode, result.ChineseName
			if code == "" {
				code = "UNKNOWN"
			}
			if name == "" {
				name = "未知地区"
			}
			if measured {
				label = fmt.Sprintf("%s %s (%.0fms  %.0fMB/s)", code, name, result.LatencyMs, result.DownloadSpeed)
			} else {
				label = fmt.Sprintf("%s %s (%.0fms)", code, name, result.LatencyMs)
			}
		}
		if _, err := fmt.Fprintf(destination, "%s#%s\n", address, label); err != nil {
			return err
		}
	}
	return nil
}
