package main

import (
	"bufio"
	"cmp"
	"crypto/tls"
	"fmt"
	"net"
	"slices"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// SpeedResult 测速结果
type SpeedResult struct {
	IP            string  `json:"ip"`
	Port          int     `json:"port"`
	LatencyMs     float64 `json:"latencyMs"`
	DownloadSpeed float64 `json:"downloadSpeed"`
	IataCode      string  `json:"iataCode"`
	CountryCode   string  `json:"countryCode"`
	ChineseName   string  `json:"chineseName"`
	TestType      string  `json:"testType"`
}

// SpeedTester 测速器（串行版本）
type SpeedTester struct {
	AllResults []ScanResult
	Regions    []string
	MaxCount   int
	Port       int
	stopped    atomic.Bool
	OnLog      func(msg string)
	OnProgress func(current, total int)
	OnComplete func(results []SpeedResult)
}

// NewSpeedTester 创建测速器
func NewSpeedTester(allResults []ScanResult, regionCode string, maxCount, port int) *SpeedTester {
	var regions []string
	if regionCode != "" {
		regions = []string{strings.ToUpper(regionCode)}
	}
	return newSpeedTester(allResults, regions, maxCount, port)
}

func newSpeedTester(allResults []ScanResult, regions []string, maxCount, port int) *SpeedTester {
	return &SpeedTester{
		AllResults: allResults,
		Regions:    regions,
		MaxCount:   maxCount,
		Port:       port,
	}
}

// Stop 停止测速
func (st *SpeedTester) Stop() {
	st.stopped.Store(true)
}

func (st *SpeedTester) targets() []ScanResult {
	sorted := slices.Clone(st.AllResults)
	sortResultsByLatency(sorted)
	if len(st.Regions) == 0 {
		return sorted
	}
	byRegion := make(map[string][]ScanResult, len(st.Regions))
	for _, region := range st.Regions {
		byRegion[region] = nil
	}
	for _, result := range sorted {
		region := strings.ToUpper(result.IataCode)
		if queue, selected := byRegion[region]; selected {
			byRegion[region] = append(queue, result)
		}
	}
	// 轮流从各地区取延迟最低的剩余条目，使较小的总数也能覆盖多个地区。
	limit := min(st.MaxCount, len(sorted))
	targets := make([]ScanResult, 0, limit)
	active := slices.Clone(st.Regions)
	for len(active) > 0 {
		next := active[:0]
		for _, region := range active {
			queue := byRegion[region]
			if len(queue) == 0 {
				continue
			}
			if len(targets) == limit {
				return targets
			}
			targets = append(targets, queue[0])
			if len(queue) > 1 {
				byRegion[region] = queue[1:]
				next = append(next, region)
			}
		}
		active = next
	}
	return targets
}

// Run 执行测速（串行，一个一个测）
func (st *SpeedTester) Run() {
	st.run(st.downloadSpeedSocket)
}

func (st *SpeedTester) run(download func(ip string, port, timeoutSec int) float64) {
	targets := st.targets()
	testType := "完全测速"
	if len(st.Regions) > 0 {
		testType = "地区测速"
		st.log(fmt.Sprintf("地区测速 %s，合计最多 %d 项", strings.Join(st.Regions, "、"), st.MaxCount))
	}

	total := len(targets)
	st.log(fmt.Sprintf("将对 %d 个地址与端口组合进行测速", total))

	speedResults := make([]SpeedResult, 0)

	for idx, target := range targets {
		if st.stopped.Load() {
			break
		}
		port := target.Port
		if st.Port != 0 {
			port = st.Port
		}
		// 测速：下载 50MB，超时 3 秒
		speed := download(target.IP, port, 3)
		iata := target.IataCode
		if iata == "" {
			iata = "UNKNOWN"
		}
		chName := GetChineseName(iata)
		sr := SpeedResult{
			IP:            target.IP,
			Port:          port,
			LatencyMs:     target.LatencyMs,
			DownloadSpeed: speed,
			IataCode:      iata,
			CountryCode:   countryByColo[strings.ToUpper(iata)],
			ChineseName:   chName,
			TestType:      testType,
		}
		speedResults = append(speedResults, sr)
		if st.OnProgress != nil {
			st.OnProgress(idx+1, total)
		}
		if iata == "UNKNOWN" {
			chName = "未识别"
		}
		st.log(fmt.Sprintf("[%d/%d] %s · %.2f MB/s · 地区 %s", idx+1, total,
			net.JoinHostPort(target.IP, strconv.Itoa(port)), speed, chName))
		// 测完一个稍作等待，避免服务端限流
		if idx+1 < total && !st.stopped.Load() {
			time.Sleep(300 * time.Millisecond)
		}
	}

	sortResultsBySpeedDesc(speedResults)
	if st.OnComplete != nil {
		st.OnComplete(speedResults)
	}
}

// downloadSpeedSocket 使用原始 socket 测速（硬编码 Cloudflare 默认地址）
func (st *SpeedTester) downloadSpeedSocket(ip string, port int, timeoutSec int) float64 {
	// 构造请求（使用默认的 Cloudflare 测速地址）
	req := fmt.Sprintf("GET /__down?bytes=50000000 HTTP/1.1\r\nHost: speed.cloudflare.com\r\nUser-Agent: Mozilla/5.0\r\nAccept: */*\r\nConnection: close\r\n\r\n")

	var conn net.Conn
	var err error

	// 建立连接（支持 IPv6）
	addr := fmt.Sprintf("%s:%d", ip, port)
	if strings.Contains(ip, ":") {
		addr = fmt.Sprintf("[%s]:%d", ip, port)
	}

	// 尝试 TLS 或普通 TCP
	useHTTPS := isCloudflareHTTPSPort(port)
	for _, useTLS := range []bool{useHTTPS, false} {
		if useTLS {
			dialer := &net.Dialer{Timeout: 3 * time.Second}
			conn, err = tls.DialWithDialer(dialer, "tcp", addr, &tls.Config{
				InsecureSkipVerify: true,
				ServerName:         "speed.cloudflare.com",
			})
		} else {
			conn, err = net.DialTimeout("tcp", addr, 3*time.Second)
		}
		if err == nil {
			break
		}
	}
	if err != nil {
		return 0
	}
	defer conn.Close()

	// 发送请求
	_, err = conn.Write([]byte(req))
	if err != nil {
		return 0
	}

	// 设置读取超时（整个测速过程不超过 timeoutSec+1 秒）
	conn.SetReadDeadline(time.Now().Add(time.Duration(timeoutSec+1) * time.Second))

	// 读取响应头
	reader := bufio.NewReader(conn)
	// 跳过所有头部直到空行
	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			return 0
		}
		if line == "\r\n" || line == "\n" {
			break
		}
	}

	// 开始读取 body 并计时
	start := time.Now()
	var downloaded int64
	buf := make([]byte, 8192) // 8KB buffer

	for {
		// 检查是否超时
		if time.Since(start).Seconds() > float64(timeoutSec) {
			break
		}
		n, err := reader.Read(buf)
		if n > 0 {
			downloaded += int64(n)
		}
		if err != nil {
			break
		}
	}
	elapsed := time.Since(start).Seconds()
	if elapsed < 0.1 {
		elapsed = 0.1
	}
	speed := float64(downloaded) / (1024 * 1024) / elapsed
	return speed
}

func (st *SpeedTester) log(msg string) {
	if st.OnLog != nil {
		st.OnLog(msg)
	}
}

// 辅助排序函数
func sortResultsByLatency(results []ScanResult) {
	slices.SortStableFunc(results, func(a, b ScanResult) int { return cmp.Compare(a.LatencyMs, b.LatencyMs) })
}

func sortResultsBySpeedDesc(results []SpeedResult) {
	slices.SortStableFunc(results, func(a, b SpeedResult) int { return cmp.Compare(b.DownloadSpeed, a.DownloadSpeed) })
}
