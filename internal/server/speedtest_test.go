package server

import (
	"context"
	"testing"
	"time"
)

func TestParseSpeedTest(t *testing.T) {
	result, err := parseSpeedTest([]byte(`{"timestamp":"2026-11-07T10:00:00Z","server":{"name":"Test Server"},"client":{"ip":"203.0.113.10"},"bytes_sent":1000,"bytes_received":2000,"ping":12.5,"jitter":1.2,"upload":100,"download":500}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.ServerName != "Test Server" || result.DownloadMbps != 500 || result.UploadMbps != 100 || result.BytesReceived != 2000 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestParseSpeedTestJSONArray(t *testing.T) {
	result, err := parseSpeedTest([]byte(`[{"timestamp":"2026-11-07T10:00:00Z","server":{"name":"Array Server"},"bytes_sent":1000,"bytes_received":2000,"ping":12.5,"jitter":1.2,"upload":100,"download":500}]`))
	if err != nil {
		t.Fatal(err)
	}
	if result.ServerName != "Array Server" || result.DownloadMbps != 500 {
		t.Fatalf("unexpected array result: %#v", result)
	}
}

func TestParseSpeedTestRejectsInvalidData(t *testing.T) {
	if _, err := parseSpeedTest([]byte(`{"download":-1}`)); err == nil {
		t.Fatal("expected negative speed to fail")
	}
	if _, err := parseSpeedTest([]byte(`not-json`)); err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
	if _, err := parseSpeedTest([]byte(`[]`)); err == nil {
		t.Fatal("expected empty result to fail")
	}
}

func TestSpeedTesterPreventsConcurrentRuns(t *testing.T) {
	tester := NewSpeedTester()
	finished := make(chan struct{})
	tester.run = func(context.Context) (SpeedTestResult, error) {
		<-finished
		return SpeedTestResult{ServerName: "done", DownloadMbps: 100}, nil
	}
	if !tester.Start() {
		t.Fatal("first run should start")
	}
	if tester.Start() {
		t.Fatal("concurrent run should be rejected")
	}
	close(finished)
	deadline := time.Now().Add(2 * time.Second)
	for tester.State().Running && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	state := tester.State()
	if state.Running || state.Result == nil || state.Result.ServerName != "done" {
		t.Fatalf("unexpected final state: %#v", state)
	}
}

func TestSpeedTestLiveState(t *testing.T) {
	tester := NewSpeedTester()
	tester.running = true
	tester.updateLive(512.4, 128.2)
	state := tester.State()
	if state.Live == nil || state.Live.DownloadMbps != 512.4 || state.Live.UploadMbps != 128.2 {
		t.Fatalf("unexpected live state: %#v", state.Live)
	}
}
