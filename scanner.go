package main

import (
	"bufio"
	"crypto/tls"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"
)

type ScanResult struct {
	IP          string  `json:"ip"`
	Port        int     `json:"port"`
	LatencyMs   float64 `json:"latencyMs"`
	IataCode    string  `json:"iataCode"`
	CountryCode string  `json:"countryCode"`
	ChineseName string  `json:"chineseName"`
	Success     bool    `json:"success"`
}

type Scanner struct {
	Version          int
	Ports            []int
	MaxWorkers       int
	LatencyThreshold int
	V4Sample         int
	V6Sample         int
	OnLog            func(msg string)
	OnProgress       func(current, total, found int, speed float64)
	OnResult         func(result ScanResult)
	stopOnce         sync.Once
	stopped          chan struct{}
}

func NewScanner(version, port, maxWorkers, latencyThreshold, v4Sample, v6Sample int) *Scanner {
	return newScanner(version, []int{port}, maxWorkers, latencyThreshold, v4Sample, v6Sample)
}

func isCloudflareHTTPSPort(port int) bool {
	switch port {
	case 443, 2053, 2083, 2087, 2096, 8443:
		return true
	default:
		return false
	}
}

func newScanner(version int, ports []int, maxWorkers, latencyThreshold, v4Sample, v6Sample int) *Scanner {
	return &Scanner{
		Version:          version,
		Ports:            append([]int(nil), ports...),
		MaxWorkers:       max(1, maxWorkers),
		LatencyThreshold: latencyThreshold,
		V4Sample:         v4Sample,
		V6Sample:         v6Sample,
		stopped:          make(chan struct{}),
	}
}

func (s *Scanner) Stop() {
	s.stopOnce.Do(func() { close(s.stopped) })
}

func (s *Scanner) Run() []ScanResult {
	s.log(fmt.Sprintf("正在从 Cloudflare IPv%d 网段生成随机 IP（端口 %v）", s.Version, s.Ports))
	ips := s.generateIPs()
	if len(ips) == 0 {
		s.log(fmt.Sprintf("错误: 未能生成IPv%d IP列表", s.Version))
		return []ScanResult{}
	}
	s.log(fmt.Sprintf("开始测试 %d 个 IP，共 %d 个地址与端口组合", len(ips), len(ips)*len(s.Ports)))
	results := s.scanTargets(ips, s.probe)
	s.log(fmt.Sprintf("扫描结束，共发现 %d 个可用地址与端口组合", len(results)))
	return results
}

func (s *Scanner) scanTargets(ips []string, probe func(ip string, port int) ScanResult) []ScanResult {
	type target struct {
		ip   string
		port int
	}
	targets := make(chan target)
	results := []ScanResult{}
	var mu sync.Mutex
	var wg sync.WaitGroup
	total := len(ips) * len(s.Ports)
	completed := 0
	startTime := time.Now()

	for range min(s.MaxWorkers, total) {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for candidate := range targets {
				select {
				case <-s.stopped:
					return
				default:
				}
				result := probe(candidate.ip, candidate.port)
				mu.Lock()
				completed++
				if result.Success {
					results = append(results, result)
					if s.OnResult != nil {
						s.OnResult(result)
					}
				}
				s.updateProgress(completed, total, len(results), startTime)
				mu.Unlock()
			}
		}()
	}

enqueue:
	for _, ip := range ips {
		for _, port := range s.Ports {
			select {
			case <-s.stopped:
				break enqueue
			case targets <- target{ip: ip, port: port}:
			}
		}
	}
	close(targets)
	wg.Wait()
	return results
}

func (s *Scanner) probe(ip string, port int) ScanResult {
	latency := s.tcpPing(ip, port, time.Second)
	if latency == 0 || latency > float64(s.LatencyThreshold) {
		return ScanResult{}
	}
	iata := s.getIataCodeSocket(ip, port)
	name := GetChineseName(iata)
	if iata == "" {
		iata = "UNKNOWN"
		name = "未知地区"
	}
	return ScanResult{
		IP: ip, Port: port, LatencyMs: latency,
		IataCode: iata, CountryCode: countryByColo[iata], ChineseName: name, Success: true,
	}
}

// generateIPs 传递采样参数
func (s *Scanner) generateIPs() []string {
	if s.Version == 4 {
		return generateIPv4List(s.V4Sample)
	}
	return generateIPv6List(s.V6Sample)
}

func (s *Scanner) tcpPing(ip string, port int, timeout time.Duration) float64 {
	start := time.Now()
	addr := fmt.Sprintf("%s:%d", ip, port)
	if strings.Contains(ip, ":") {
		addr = fmt.Sprintf("[%s]:%d", ip, port)
	}
	conn, err := net.DialTimeout("tcp", addr, timeout)
	if err != nil {
		return 0
	}
	defer conn.Close()
	return float64(time.Since(start).Microseconds()) / 1000.0
}

func (s *Scanner) getIataCodeSocket(ip string, port int) string {
	for _, useTLS := range []bool{true, false} {
		addr := fmt.Sprintf("%s:%d", ip, port)
		if strings.Contains(ip, ":") {
			addr = fmt.Sprintf("[%s]:%d", ip, port)
		}
		var conn net.Conn
		var err error
		if useTLS {
			dialer := &net.Dialer{Timeout: 2 * time.Second}
			conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         "speed.cloudflare.com",
			})
		} else {
			conn, err = net.DialTimeout("tcp", addr, 2*time.Second)
		}
		if err != nil {
			continue
		}
		conn.SetDeadline(time.Now().Add(3 * time.Second))
		const req = "GET /cdn-cgi/trace HTTP/1.1\r\nHost: speed.cloudflare.com\r\nUser-Agent: Mozilla/5.0\r\nConnection: close\r\n\r\n"
		_, err = conn.Write([]byte(req))
		if err != nil {
			conn.Close()
			continue
		}
		code := readCloudflareColo(conn)
		conn.Close()
		if code != "" {
			return code
		}
	}
	return ""
}

func readCloudflareColo(reader io.Reader) string {
	response, err := http.ReadResponse(bufio.NewReader(reader), nil)
	if err != nil {
		return ""
	}
	if response.StatusCode == http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(response.Body, 16<<10))
		for _, line := range strings.Split(string(body), "\n") {
			if value, ok := strings.CutPrefix(line, "colo="); ok {
				if code := recognizedRegionCode(value); code != "" {
					return code
				}
			}
		}
	}
	// Cf-Ray 的末尾是处理该次请求的数据中心码。
	ray, suffix, ok := strings.Cut(strings.TrimSpace(response.Header.Get("Cf-Ray")), "-")
	if !ok || len(ray) < 8 {
		return ""
	}
	for _, digit := range ray {
		if !strings.ContainsRune("0123456789abcdefABCDEF", digit) {
			return ""
		}
	}
	return recognizedRegionCode(suffix)
}

func (s *Scanner) updateProgress(current, total, found int, startTime time.Time) {
	if s.OnProgress == nil {
		return
	}
	elapsed := time.Since(startTime).Seconds()
	speed := float64(current) / elapsed
	s.OnProgress(current, total, found, speed)
}

func (s *Scanner) log(msg string) {
	if s.OnLog != nil {
		s.OnLog(msg)
	}
}
