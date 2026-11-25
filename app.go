package main

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

type App struct {
	ctx                context.Context
	mu                 sync.Mutex
	currentScanner     *Scanner
	currentSpeedTester *SpeedTester
	lastScanResults    []ScanResult
	lastSpeedResults   []SpeedResult
	lastExportMillis   int64
}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) StartScan(version, port, maxWorkers, latencyThreshold, v4Sample, v6Sample int) {
	if err := a.startScan(NewScanner(version, port, maxWorkers, latencyThreshold, v4Sample, v6Sample)); err != nil {
		a.EmitLog(err.Error())
	}
}

func (a *App) StartMultiPortScan(version int, ports []int, maxWorkers, latencyThreshold, v4Sample, v6Sample int) error {
	if version != 4 && version != 6 {
		return fmt.Errorf("请选择 IPv4 或 IPv6")
	}
	selected := make([]int, 0, 6)
	for _, port := range ports {
		if !isCloudflareHTTPSPort(port) {
			return fmt.Errorf("端口 %d 不在可选范围内", port)
		}
		if !slices.Contains(selected, port) {
			selected = append(selected, port)
		}
	}
	if len(selected) == 0 {
		return fmt.Errorf("请至少勾选一个端口")
	}
	return a.startScan(newScanner(version, selected, maxWorkers, latencyThreshold, v4Sample, v6Sample))
}

func (a *App) startScan(scanner *Scanner) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.currentScanner != nil || a.currentSpeedTester != nil {
		return fmt.Errorf("已有任务正在执行，请先停止")
	}
	a.currentScanner = scanner
	a.lastScanResults = nil
	a.lastSpeedResults = nil
	a.EmitLog(fmt.Sprintf("开始 IPv%d 扫描，并发 %d，延迟上限 %d ms，V4 采样 %d，V6 采样 %d",
		scanner.Version, scanner.MaxWorkers, scanner.LatencyThreshold, scanner.V4Sample, scanner.V6Sample))
	scanner.OnLog = a.EmitLog
	scanner.OnProgress = func(current, total, found int, speed float64) {
		runtime.EventsEmit(a.ctx, "scan-progress", current, total, found, speed)
	}
	scanner.OnResult = func(result ScanResult) {
		runtime.EventsEmit(a.ctx, "scan-result", result)
	}
	go func() {
		results := scanner.Run()
		a.mu.Lock()
		defer a.mu.Unlock()
		a.lastScanResults = results
		a.currentScanner = nil
		runtime.EventsEmit(a.ctx, "scan-completed", results)
	}()
	return nil
}

func (a *App) StopScan() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.currentScanner != nil {
		a.currentScanner.Stop()
		a.EmitLog("正在停止扫描...")
	}
}

func (a *App) StartFullSpeedTest(_ int, port int) {
	if err := a.startSpeedTest(nil, 0, port); err != nil {
		a.EmitLog(err.Error())
	}
}

func (a *App) StartRegionSpeedTest(regionCode string, maxCount, port int) {
	var regions []string
	if regionCode != "" {
		regions = []string{strings.ToUpper(regionCode)}
	}
	if err := a.startSpeedTest(regions, maxCount, port); err != nil {
		a.EmitLog(err.Error())
	}
}

// StartScannedSpeedTest 保留原有调用入口。
func (a *App) StartScannedSpeedTest(_ int) error {
	return a.StartAllScannedSpeedTest()
}

func (a *App) StartAllScannedSpeedTest() error {
	return a.startSpeedTest(nil, 0, 0)
}

func (a *App) StartMultiRegionSpeedTest(regionCodes []string, maxCount int) error {
	regions := normalizedRegions(regionCodes)
	if len(regions) == 0 {
		return fmt.Errorf("请至少勾选一个地区")
	}
	return a.startSpeedTest(regions, maxCount, 0)
}

func normalizedRegions(regionCodes []string) []string {
	regions := make([]string, 0, len(regionCodes))
	for _, raw := range regionCodes {
		if code := recognizedRegionCode(raw); code != "" && !slices.Contains(regions, code) {
			regions = append(regions, code)
		}
	}
	return regions
}

func (a *App) startSpeedTest(regions []string, maxCount, port int) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.currentScanner != nil || a.currentSpeedTester != nil {
		return fmt.Errorf("请先停止当前任务")
	}
	if len(a.lastScanResults) == 0 {
		return fmt.Errorf("没有可用的 IP，请先进行扫描")
	}
	if len(regions) > 0 {
		limit := 0
		for _, result := range a.lastScanResults {
			if slices.Contains(regions, strings.ToUpper(result.IataCode)) {
				limit++
			}
		}
		if limit == 0 {
			return fmt.Errorf("所选地区没有可用结果")
		}
		if maxCount < 1 || maxCount > limit {
			return fmt.Errorf("测速数量必须在 1–%d 之间", limit)
		}
	}

	tester := newSpeedTester(a.lastScanResults, regions, maxCount, port)
	a.currentSpeedTester = tester
	a.lastSpeedResults = nil
	tester.OnLog = a.EmitLog
	tester.OnProgress = func(current, total int) {
		runtime.EventsEmit(a.ctx, "speed-progress", current, total)
	}
	tester.OnComplete = func(results []SpeedResult) {
		a.mu.Lock()
		defer a.mu.Unlock()
		a.currentSpeedTester = nil
		a.lastSpeedResults = results
		runtime.EventsEmit(a.ctx, "speed-completed", results)
	}
	go tester.Run()
	return nil
}

func (a *App) StopSpeedTest() {
	a.mu.Lock()
	defer a.mu.Unlock()
	if a.currentSpeedTester != nil {
		a.currentSpeedTester.Stop()
		a.EmitLog("正在停止测速...")
	}
}

func (a *App) EmitLog(msg string) {
	runtime.EventsEmit(a.ctx, "log-message", msg)
}

func (a *App) GetChineseName(iataCode string) string {
	return GetChineseName(iataCode)
}
