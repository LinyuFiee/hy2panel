package server

import (
	"context"
	"testing"
	"time"
)

func TestParseSpeedTest(t *testing.T) {
	result, err := parseSpeedTest([]byte(`{"timestamp":"2026-11-07T10:00:00Z","server":{"name":"Test Server"},"client":{"ip":"203.0.113.10"},"bytes_sent":1000,"bytes_received":2000,"ping":12.5,"jitter":1.2,"upload":100000000,"download":500000000}`))
	if err != nil {
		t.Fatal(err)
	}
	if result.ServerName != "Test Server" || result.DownloadBps != 500000000 || result.UploadBps != 100000000 || result.BytesReceived != 2000 {
		t.Fatalf("unexpected result: %#v", result)
	}
}

func TestParseSpeedTestRejectsInvalidData(t *testing.T) {
	if _, err := parseSpeedTest([]byte(`{"download":-1}`)); err == nil {
		t.Fatal("expected negative speed to fail")
	}
	if _, err := parseSpeedTest([]byte(`not-json`)); err == nil {
		t.Fatal("expected invalid JSON to fail")
	}
}

func TestSpeedTesterPreventsConcurrentRuns(t *testing.T) {
	tester := NewSpeedTester()
	finished := make(chan struct{})
	tester.run = func(context.Context) (SpeedTestResult, error) {
		<-finished
		return SpeedTestResult{ServerName: "done", DownloadBps: 100}, nil
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
