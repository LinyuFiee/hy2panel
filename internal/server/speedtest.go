package server

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os/exec"
	"strings"
	"sync"
	"time"
)

type SpeedTestResult struct {
	Timestamp     time.Time `json:"timestamp"`
	ServerName    string    `json:"server_name"`
	Ping          float64   `json:"ping"`
	Jitter        float64   `json:"jitter"`
	DownloadMbps  float64   `json:"download_mbps"`
	UploadMbps    float64   `json:"upload_mbps"`
	BytesSent     uint64    `json:"bytes_sent"`
	BytesReceived uint64    `json:"bytes_received"`
}

type SpeedTestLive struct {
	DownloadMbps float64 `json:"download_mbps"`
	UploadMbps   float64 `json:"upload_mbps"`
}

type SpeedTestState struct {
	Running bool             `json:"running"`
	Live    *SpeedTestLive   `json:"live,omitempty"`
	Result  *SpeedTestResult `json:"result,omitempty"`
	Error   string           `json:"error,omitempty"`
}

type SpeedTester struct {
	mu        sync.Mutex
	running   bool
	live      *SpeedTestLive
	result    *SpeedTestResult
	lastError string
	binary    string
	run       func(context.Context) (SpeedTestResult, error)
}

func NewSpeedTester() *SpeedTester {
	tester := &SpeedTester{binary: "/usr/local/bin/librespeed-cli"}
	tester.run = tester.execute
	return tester
}

func (t *SpeedTester) Start() bool {
	t.mu.Lock()
	if t.running {
		t.mu.Unlock()
		return false
	}
	t.running = true
	t.live = nil
	t.lastError = ""
	t.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
		defer cancel()
		result, err := t.run(ctx)
		t.mu.Lock()
		defer t.mu.Unlock()
		t.running = false
		t.live = nil
		if err != nil {
			t.lastError = err.Error()
			return
		}
		t.result = &result
		t.lastError = ""
	}()
	return true
}

func (t *SpeedTester) State() SpeedTestState {
	t.mu.Lock()
	defer t.mu.Unlock()
	state := SpeedTestState{Running: t.running, Error: t.lastError}
	if t.live != nil {
		live := *t.live
		state.Live = &live
	}
	if t.result != nil {
		result := *t.result
		state.Result = &result
	}
	return state
}

func (t *SpeedTester) execute(ctx context.Context) (SpeedTestResult, error) {
	command := exec.CommandContext(ctx, t.binary, "--json", "--secure", "--no-icmp", "--concurrent", "3", "--duration", "8", "--timeout", "15")
	var stdout, stderr bytes.Buffer
	command.Stdout = &stdout
	command.Stderr = &stderr
	if err := command.Start(); err != nil {
		return SpeedTestResult{}, fmt.Errorf("无法启动测速工具: %w", err)
	}
	done := make(chan error, 1)
	go func() { done <- command.Wait() }()
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	var previousRX, previousTX uint64
	previousTime := time.Now()
	if rx, tx, ok := networkTotals(); ok {
		previousRX, previousTX = rx, tx
	}
	for {
		select {
		case err := <-done:
			if err != nil {
				message := strings.TrimSpace(stderr.String())
				if len(message) > 1000 {
					message = message[:1000]
				}
				if message == "" {
					message = err.Error()
				}
				return SpeedTestResult{}, fmt.Errorf("测速失败: %s", message)
			}
			return parseSpeedTest(stdout.Bytes())
		case now := <-ticker.C:
			rx, tx, ok := networkTotals()
			if !ok {
				continue
			}
			elapsed := now.Sub(previousTime).Seconds()
			if elapsed > 0 && rx >= previousRX && tx >= previousTX {
				t.updateLive(float64(rx-previousRX)*8/elapsed/1_000_000, float64(tx-previousTX)*8/elapsed/1_000_000)
			}
			previousRX, previousTX, previousTime = rx, tx, now
		}
	}
}

func (t *SpeedTester) updateLive(download, upload float64) {
	t.mu.Lock()
	t.live = &SpeedTestLive{DownloadMbps: download, UploadMbps: upload}
	t.mu.Unlock()
}

type speedTestPayload struct {
	Timestamp time.Time `json:"timestamp"`
	Server    struct {
		Name string `json:"name"`
	} `json:"server"`
	BytesSent     uint64  `json:"bytes_sent"`
	BytesReceived uint64  `json:"bytes_received"`
	Ping          float64 `json:"ping"`
	Jitter        float64 `json:"jitter"`
	Upload        float64 `json:"upload"`
	Download      float64 `json:"download"`
}

func parseSpeedTest(output []byte) (SpeedTestResult, error) {
	trimmed := bytes.TrimSpace(output)
	if len(trimmed) == 0 {
		return SpeedTestResult{}, errors.New("测速工具没有返回结果")
	}
	var payloads []speedTestPayload
	switch trimmed[0] {
	case '[':
		if err := json.Unmarshal(trimmed, &payloads); err != nil {
			return SpeedTestResult{}, errors.New("测速结果不是有效的 JSON 数组")
		}
	case '{':
		var payload speedTestPayload
		if err := json.Unmarshal(trimmed, &payload); err != nil {
			return SpeedTestResult{}, errors.New("测速结果不是有效的 JSON 对象")
		}
		payloads = []speedTestPayload{payload}
	default:
		return SpeedTestResult{}, errors.New("测速结果不是有效的 JSON")
	}
	if len(payloads) == 0 {
		return SpeedTestResult{}, errors.New("测速未返回可用服务器结果")
	}
	payload := payloads[len(payloads)-1]
	if payload.Download < 0 || payload.Upload < 0 || payload.Ping < 0 || payload.Jitter < 0 {
		return SpeedTestResult{}, errors.New("测速结果包含无效数值")
	}
	return SpeedTestResult{
		Timestamp:     payload.Timestamp,
		ServerName:    payload.Server.Name,
		Ping:          payload.Ping,
		Jitter:        payload.Jitter,
		DownloadMbps:  payload.Download,
		UploadMbps:    payload.Upload,
		BytesSent:     payload.BytesSent,
		BytesReceived: payload.BytesReceived,
	}, nil
}

func (s *Server) startSpeedTest(w http.ResponseWriter, r *http.Request) {
	if !validMutation(r) {
		writeError(w, http.StatusForbidden, "请求来源无效")
		return
	}
	if !s.speedtest.Start() {
		writeError(w, http.StatusConflict, "已有测速任务正在运行")
		return
	}
	writeJSON(w, http.StatusAccepted, s.speedtest.State())
}
