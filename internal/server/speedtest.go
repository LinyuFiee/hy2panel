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
	DownloadBps   float64   `json:"download_bps"`
	UploadBps     float64   `json:"upload_bps"`
	BytesSent     uint64    `json:"bytes_sent"`
	BytesReceived uint64    `json:"bytes_received"`
}

type SpeedTestState struct {
	Running bool             `json:"running"`
	Result  *SpeedTestResult `json:"result,omitempty"`
	Error   string           `json:"error,omitempty"`
}

type SpeedTester struct {
	mu        sync.Mutex
	running   bool
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
	t.lastError = ""
	t.mu.Unlock()
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 75*time.Second)
		defer cancel()
		result, err := t.run(ctx)
		t.mu.Lock()
		defer t.mu.Unlock()
		t.running = false
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
	if err := command.Run(); err != nil {
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
}

func parseSpeedTest(output []byte) (SpeedTestResult, error) {
	var payload struct {
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
	if err := json.Unmarshal(output, &payload); err != nil {
		return SpeedTestResult{}, errors.New("测速结果不是有效 JSON")
	}
	if payload.Download < 0 || payload.Upload < 0 || payload.Ping < 0 || payload.Jitter < 0 {
		return SpeedTestResult{}, errors.New("测速结果包含无效数值")
	}
	return SpeedTestResult{
		Timestamp:     payload.Timestamp,
		ServerName:    payload.Server.Name,
		Ping:          payload.Ping,
		Jitter:        payload.Jitter,
		DownloadBps:   payload.Download,
		UploadBps:     payload.Upload,
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
