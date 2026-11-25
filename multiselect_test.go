package main

import (
	"reflect"
	"sync/atomic"
	"testing"
)

func TestScanTargetsTestsEverySelectedPort(t *testing.T) {
	scanner := newScanner(4, []int{443, 8443}, 2, 220, 1, 100)
	var progress [3]int
	scanner.OnProgress = func(done, total, found int, _ float64) {
		progress = [3]int{done, total, found}
	}
	ips := []string{"192.0.2.1", "192.0.2.2"}
	var checked443 atomic.Int32
	results := scanner.scanTargets(ips, func(ip string, port int) ScanResult {
		if port == 443 {
			checked443.Add(1)
		}
		return ScanResult{IP: ip, Port: port, Success: port == 8443 || ip == ips[0]}
	})
	if progress != [3]int{4, 4, 3} || checked443.Load() != 2 || len(results) != 3 {
		t.Fatalf("progress = %v, 443 attempts = %d, results = %+v", progress, checked443.Load(), results)
	}
}

func TestScanTargetsStopsBeforeDispatch(t *testing.T) {
	scanner := newScanner(6, []int{443, 8443}, 2, 220, 1, 100)
	scanner.Stop()
	scanner.Stop()
	results := scanner.scanTargets([]string{"2001:db8::1"}, func(ip string, port int) ScanResult {
		t.Errorf("stopped scan probed %s port %d", ip, port)
		return ScanResult{}
	})
	if len(results) != 0 {
		t.Fatalf("stopped scan returned %+v", results)
	}
}

func TestRegionSpeedUsesEachResultPortAndCombinedLimit(t *testing.T) {
	results := []ScanResult{
		{IP: "192.0.2.1", IataCode: "HKG", Port: 443, LatencyMs: 50},
		{IP: "192.0.2.1", IataCode: "HKG", Port: 8443, LatencyMs: 20},
		{IP: "192.0.2.2", IataCode: "SIN", Port: 2053, LatencyMs: 60},
		{IP: "192.0.2.3", IataCode: "NRT", Port: 443, LatencyMs: 10},
	}
	tester := newSpeedTester(results, []string{"HKG", "SIN"}, 2, 0)
	var ports []int
	var measured []SpeedResult
	tester.OnComplete = func(result []SpeedResult) { measured = result }
	tester.run(func(_ string, port, _ int) float64 {
		ports = append(ports, port)
		return 10
	})
	if !reflect.DeepEqual(ports, []int{8443, 2053}) {
		t.Fatalf("measured ports = %v", ports)
	}
	if len(measured) != 2 || measured[0].Port != ports[0] || measured[1].Port != ports[1] {
		t.Fatalf("speed results = %+v", measured)
	}
	if results[0].Port != 443 {
		t.Fatalf("source scan results reordered: %+v", results)
	}
	if got := newSpeedTester(results, []string{"HKG", "SIN"}, 1, 0).targets(); len(got) != 1 || got[0].Port != 8443 {
		t.Fatalf("one regional target = %+v", got)
	}
	if got := newSpeedTester(results, []string{"HKG", "SIN"}, 3, 0).targets(); len(got) != 3 || got[2].Port != 443 {
		t.Fatalf("all selected regional targets = %+v", got)
	}
	app := &App{lastScanResults: results}
	if err := app.StartMultiRegionSpeedTest([]string{"HKG", "SIN"}, 4); err == nil {
		t.Fatal("accepted more than the three results available in the selected regions")
	}
}

func TestFullSpeedTestUsesAllScannedResults(t *testing.T) {
	results := make([]ScanResult, 503)
	for index := range results {
		results[index] = ScanResult{IataCode: "SIN", Port: 443, LatencyMs: float64(len(results) - index)}
	}
	results[len(results)-1].IataCode = "UNKNOWN"
	results[len(results)-1].Port = 8443
	targets := newSpeedTester(results, nil, 10, 0).targets()
	if len(targets) != len(results) || targets[0].IataCode != "UNKNOWN" || targets[0].Port != 8443 {
		t.Fatalf("full speed selected %d of %d scan results", len(targets), len(results))
	}
	if results[0].LatencyMs != float64(len(results)) {
		t.Fatal("full speed changed the original scan order")
	}
}
