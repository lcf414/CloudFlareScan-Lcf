package main

import (
	"bytes"
	"encoding/csv"
	"reflect"
	"strings"
	"testing"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
	"golang.org/x/text/transform"
)

func TestExportFormatsAndLayouts(t *testing.T) {
	results := []SpeedResult{{
		IP: "2001:db8::1", Port: 8443, IataCode: "SIN", ChineseName: "新加坡",
		LatencyMs: 35, DownloadSpeed: 12, TestType: "地区测速",
	}}
	for _, scenario := range []struct {
		name              string
		compact, measured bool
		csvHeader, csvRow []string
		txt               string
	}{
		{"测过默认", false, true,
			[]string{"排名", "IP地址", "地区码", "地区", "延迟(ms)", "下载速度(MB/s)", "端口", "测速类型"},
			[]string{"1", "2001:db8::1", "SIN", "新加坡", "35.00", "12.00", "8443", "地区测速"},
			"[2001:db8::1]:8443#SIN 新加坡 (35ms  12MB/s)\n"},
		{"仅扫描默认", false, false,
			[]string{"排名", "IP地址", "地区码", "地区", "延迟(ms)", "下载速度(MB/s)", "端口", "测速类型"},
			[]string{"1", "2001:db8::1", "SIN", "新加坡", "35.00", "", "8443", "扫描"},
			"[2001:db8::1]:8443#SIN 新加坡 (35ms)\n"},
		{"测过精简", true, true,
			[]string{"排名", "IP地址", "地区码", "地区", "端口"},
			[]string{"1", "2001:db8::1", "SIN", "SG-SIN", "8443"},
			"[2001:db8::1]:8443#SG-SIN\n"},
		{"仅扫描精简", true, false,
			[]string{"排名", "IP地址", "地区码", "地区", "端口"},
			[]string{"1", "2001:db8::1", "SIN", "SG-SIN", "8443"},
			"[2001:db8::1]:8443#SG-SIN\n"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			row := results[0]
			if !scenario.measured {
				row.TestType = "扫描"
			}
			var output bytes.Buffer
			if err := writeCSVResults(&output, []SpeedResult{row}, scenario.compact, scenario.measured); err != nil {
				t.Fatal(err)
			}
			decoder := transform.NewReader(&output, simplifiedchinese.GBK.NewDecoder())
			lines, err := csv.NewReader(decoder).ReadAll()
			if err != nil || !reflect.DeepEqual(lines, [][]string{scenario.csvHeader, scenario.csvRow}) {
				t.Fatalf("CSV output = %q, error = %v", lines, err)
			}
			var text strings.Builder
			if err := writeTXTResults(&text, []SpeedResult{row}, scenario.compact, scenario.measured); err != nil || text.String() != scenario.txt {
				t.Fatalf("TXT output = %q, error = %v", text.String(), err)
			}
		})
	}
}

func TestCompactRegionLabelUsesEachRowsColo(t *testing.T) {
	results := []SpeedResult{
		{IP: "192.0.2.1", Port: 443, IataCode: "HKG"},
		{IP: "192.0.2.2", Port: 2053, IataCode: "SJC"},
		{IP: "192.0.2.3", Port: 8443, IataCode: " nrt "},
		{IP: "192.0.2.4", Port: 443, IataCode: "ZZZ"},
		{IP: "192.0.2.5", Port: 443},
	}
	var output strings.Builder
	if err := writeTXTResults(&output, results, true, false); err != nil {
		t.Fatal(err)
	}
	expected := "192.0.2.1:443#HK-HKG\n" +
		"192.0.2.2:2053#US-SJC\n" +
		"192.0.2.3:8443#JP-NRT\n" +
		"192.0.2.4:443#ZZZ\n" +
		"192.0.2.5:443#UNKNOWN\n"
	if output.String() != expected {
		t.Fatalf("compact export = %q", output.String())
	}
	for code := range airportCodes {
		if len(countryByColo[code]) != 2 {
			t.Errorf("missing country code for built-in colo %s", code)
		}
	}
	if compactRegionLabel("CDG") != "FR-CDG" || compactRegionLabel("LHR") != "GB-LHR" {
		t.Fatal("expected canonical country abbreviations for France and the UK")
	}
}

func TestCurrentExportResultsCanUseScanOnly(t *testing.T) {
	scanned := make([]ScanResult, maxExportResults+3)
	for index := range scanned {
		scanned[index] = ScanResult{IP: "192.0.2.1", Port: 443, IataCode: "SJC", LatencyMs: float64(len(scanned) - index)}
	}
	scanned = append(scanned, ScanResult{IP: "192.0.2.3", IataCode: "UNKNOWN", LatencyMs: 0.5})
	app := &App{lastScanResults: scanned}
	rows, measured := app.currentExportResults(nil)
	if measured || len(rows) != 500 || rows[0].LatencyMs != 1 || rows[499].LatencyMs != 500 || rows[0].TestType != "扫描" {
		t.Fatal("scan-only export did not select the first 500 results by latency")
	}
	app.lastSpeedResults = []SpeedResult{
		{IP: "192.0.2.3", Port: 443, IataCode: "UNKNOWN", DownloadSpeed: 20},
		{IP: "192.0.2.2", Port: 8443, IataCode: " sin ", ChineseName: "未知地区", DownloadSpeed: 12},
	}
	rows, measured = app.currentExportResults(nil)
	if !measured || len(rows) != 1 || rows[0].Port != 8443 || rows[0].DownloadSpeed != 12 || rows[0].IataCode != "SIN" || rows[0].ChineseName != "新加坡" {
		t.Fatalf("export did not prefer current speed results: %+v", rows)
	}
	app.lastSpeedResults = app.lastSpeedResults[:1]
	rows, measured = app.currentExportResults(nil)
	if measured || len(rows) != 500 || rows[0].IataCode != "SJC" {
		t.Fatalf("export did not fall back to recognized scan results: measured=%v count=%d", measured, len(rows))
	}
	app.lastScanResults = scanned[len(scanned)-1:]
	if _, err := app.ExportCurrentResults("txt", "compact"); err == nil || !strings.Contains(err.Error(), "没有识别到地区") {
		t.Fatalf("only unknown regions: error = %v", err)
	}
}

func TestExportRegionSelectionFiltersBeforeLimiting(t *testing.T) {
	scanned := make([]ScanResult, maxExportResults+1)
	measured := make([]SpeedResult, maxExportResults+1)
	for index := 0; index < maxExportResults; index++ {
		scanned[index] = ScanResult{IataCode: "SJC", LatencyMs: 10}
		measured[index] = SpeedResult{IataCode: "SJC", DownloadSpeed: 20}
	}
	scanned[maxExportResults] = ScanResult{IP: "192.0.2.1", IataCode: "SIN", LatencyMs: 50}
	measured[maxExportResults] = SpeedResult{IP: "192.0.2.2", IataCode: "SIN", DownloadSpeed: 5}
	selected := map[string]bool{"SIN": true}
	app := &App{lastScanResults: scanned}
	rows, fromSpeedTest := app.currentExportResults(selected)
	if fromSpeedTest || len(rows) != 1 || rows[0].IP != "192.0.2.1" {
		t.Fatalf("filtered scan results = %+v", rows)
	}
	app.lastSpeedResults = measured
	rows, fromSpeedTest = app.currentExportResults(selected)
	if !fromSpeedTest || len(rows) != 1 || rows[0].IP != "192.0.2.2" {
		t.Fatalf("filtered speed test results = %+v", rows)
	}
	if _, err := app.ExportCurrentRegionResults("csv", "default", nil); err == nil {
		t.Fatal("export accepted an empty region selection")
	}
}

func TestExportFileTimestampIncludesTimeAndAvoidsSameMillisecond(t *testing.T) {
	app := NewApp()
	location := time.FixedZone("UTC+8", 8*60*60)
	now := time.Date(2026, 11, 25, 23, 59, 59, 999_123_000, location)
	if got := app.exportFileTimestamp(now); got != "20261125_235959_999" {
		t.Fatalf("first export timestamp = %q", got)
	}
	if got := app.exportFileTimestamp(now); got != "20261126_000000_000" {
		t.Fatalf("same-millisecond export timestamp = %q", got)
	}
}
