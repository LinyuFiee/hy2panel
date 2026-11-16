package traffic

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"testing"
	"time"
)

var baseTestTime = time.Date(2026, 11, 16, 12, 0, 0, 0, time.UTC)

type scriptedResult struct {
	snapshot ClashSnapshot
	err      error
}

type scriptedSource struct {
	mu      sync.Mutex
	results []scriptedResult
	index   int
}

func (s *scriptedSource) Fetch(context.Context) (ClashSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.results) == 0 {
		return ClashSnapshot{}, errors.New("no scripted traffic result")
	}
	index := s.index
	if index >= len(s.results) {
		index = len(s.results) - 1
	} else {
		s.index++
	}
	return s.results[index].snapshot, s.results[index].err
}

type memoryStateStore struct {
	mu    sync.Mutex
	state PersistentState
	has   bool
	saves int
}

func (s *memoryStateStore) Load() (PersistentState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !s.has {
		return newPersistentState(), nil
	}
	return clonePersistentState(s.state), nil
}

func (s *memoryStateStore) Save(state PersistentState) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.state = clonePersistentState(state)
	s.has = true
	s.saves++
	return nil
}

func flow(id, kind string, upload, download uint64, startedAt time.Time) ClashConnection {
	return ClashConnection{
		ID:       id,
		Metadata: ClashMetadata{Type: kind},
		Upload:   upload,
		Download: download,
		Start:    startedAt,
	}
}

func snapshot(upload, download uint64, source SourceIdentity, connections ...ClashConnection) ClashSnapshot {
	return ClashSnapshot{
		UploadTotal:   upload,
		DownloadTotal: download,
		Connections:   connections,
		Source:        source,
	}
}

func newTestCollector(t *testing.T, source Source, store StateStore, now *time.Time) *Collector {
	t.Helper()
	collector, err := NewCollector(source, store, WithClock(func() time.Time { return *now }))
	if err != nil {
		t.Fatal(err)
	}
	return collector
}

func collectAt(collector *Collector, now *time.Time, at time.Time) LiveSnapshot {
	*now = at
	collector.collect(context.Background(), at)
	return collector.Snapshot()
}

func TestClassifyExactLabels(t *testing.T) {
	tests := []struct {
		label string
		want  Protocol
	}{
		{label: "vless/reality", want: ProtocolReality},
		{label: "hysteria2/hysteria2", want: ProtocolHysteria2},
		{label: "vless", want: ProtocolUnattributed},
		{label: "reality", want: ProtocolUnattributed},
		{label: "VLESS/reality", want: ProtocolUnattributed},
		{label: "hysteria2/other", want: ProtocolUnattributed},
		{label: "", want: ProtocolUnattributed},
	}
	for _, test := range tests {
		t.Run(test.label, func(t *testing.T) {
			if got := classify(test.label); got != test.want {
				t.Fatalf("classify(%q) = %q, want %q", test.label, got, test.want)
			}
		})
	}
}

func TestCollectorFirstSampleEstablishesBaseline(t *testing.T) {
	now := baseTestTime
	source := &scriptedSource{results: []scriptedResult{{snapshot: snapshot(100, 200, SourceIdentity{},
		flow("r", "vless/reality", 80, 150, baseTestTime.Add(-time.Minute)),
		flow("h", "hysteria2/hysteria2", 20, 50, baseTestTime.Add(-time.Minute)),
	)}}}
	collector := newTestCollector(t, source, nil, &now)
	view := collectAt(collector, &now, baseTestTime)
	if view.Traffic.Total() != (ByteCounters{}) || view.Total != (ByteCounters{}) {
		t.Fatalf("first sample must not count preexisting traffic: %#v", view)
	}
	if !view.Available || view.Stale || view.ActiveFlows.Reality != 1 || view.ActiveFlows.Hysteria2 != 1 {
		t.Fatalf("unexpected live baseline: %#v", view)
	}
}

func TestCollectorAttributionScenarios(t *testing.T) {
	started := baseTestTime.Add(-time.Minute)
	tests := []struct {
		name    string
		results []scriptedResult
		want    ProtocolCounters
		active  FlowCounts
	}{
		{
			name: "reality and hysteria2 exact labels",
			results: []scriptedResult{
				{snapshot: snapshot(0, 0, SourceIdentity{}, flow("r", "vless/reality", 0, 0, started), flow("h", "hysteria2/hysteria2", 0, 0, started))},
				{snapshot: snapshot(13, 27, SourceIdentity{}, flow("r", "vless/reality", 5, 7, started), flow("h", "hysteria2/hysteria2", 8, 20, started))},
			},
			want:   ProtocolCounters{Reality: ByteCounters{Upload: 5, Download: 7}, Hysteria2: ByteCounters{Upload: 8, Download: 20}},
			active: FlowCounts{Reality: 1, Hysteria2: 1},
		},
		{
			name: "unknown label remains unattributed",
			results: []scriptedResult{
				{snapshot: snapshot(0, 0, SourceIdentity{}, flow("u", "socks/mystery", 0, 0, started))},
				{snapshot: snapshot(9, 11, SourceIdentity{}, flow("u", "socks/mystery", 9, 11, started))},
			},
			want:   ProtocolCounters{Unattributed: ByteCounters{Upload: 9, Download: 11}},
			active: FlowCounts{Unattributed: 1},
		},
		{
			name: "short connection delta",
			results: []scriptedResult{
				{snapshot: snapshot(100, 100, SourceIdentity{})},
				{snapshot: snapshot(130, 140, SourceIdentity{})},
			},
			want: ProtocolCounters{Unattributed: ByteCounters{Upload: 30, Download: 40}},
		},
		{
			name: "disappeared flow tail",
			results: []scriptedResult{
				{snapshot: snapshot(10, 20, SourceIdentity{}, flow("r", "vless/reality", 10, 20, started))},
				{snapshot: snapshot(18, 29, SourceIdentity{})},
			},
			want: ProtocolCounters{Unattributed: ByteCounters{Upload: 8, Download: 9}},
		},
		{
			name: "new flow current bytes",
			results: []scriptedResult{
				{snapshot: snapshot(0, 0, SourceIdentity{})},
				{snapshot: snapshot(6, 7, SourceIdentity{}, flow("r", "vless/reality", 6, 7, started))},
			},
			want:   ProtocolCounters{Reality: ByteCounters{Upload: 6, Download: 7}},
			active: FlowCounts{Reality: 1},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := baseTestTime
			collector := newTestCollector(t, &scriptedSource{results: test.results}, nil, &now)
			collectAt(collector, &now, baseTestTime)
			view := collectAt(collector, &now, baseTestTime.Add(time.Second))
			if view.Traffic != test.want || view.Total != test.want.Total() {
				t.Fatalf("traffic = %#v, want %#v", view.Traffic, test.want)
			}
			if view.ActiveFlows != test.active {
				t.Fatalf("active flows = %#v, want %#v", view.ActiveFlows, test.active)
			}
		})
	}
}

func TestCollectorConnectionCounterDropUsesFreshBytes(t *testing.T) {
	now := baseTestTime
	started := baseTestTime.Add(-time.Minute)
	source := &scriptedSource{results: []scriptedResult{
		{snapshot: snapshot(100, 100, SourceIdentity{}, flow("r", "vless/reality", 100, 100, started))},
		{snapshot: snapshot(105, 106, SourceIdentity{}, flow("r", "vless/reality", 5, 6, started))},
	}}
	collector := newTestCollector(t, source, nil, &now)
	collectAt(collector, &now, baseTestTime)
	view := collectAt(collector, &now, baseTestTime.Add(time.Second))
	want := ProtocolCounters{Reality: ByteCounters{Upload: 5, Download: 6}}
	if view.Traffic != want {
		t.Fatalf("counter drop traffic = %#v, want %#v", view.Traffic, want)
	}
}

func TestCollectorGlobalResetAndSourceChange(t *testing.T) {
	started := baseTestTime.Add(-time.Minute)
	tests := []struct {
		name    string
		results []scriptedResult
	}{
		{
			name: "global reset",
			results: []scriptedResult{
				{snapshot: snapshot(100, 100, SourceIdentity{Instance: "node", Epoch: "one"}, flow("r", "vless/reality", 50, 50, started))},
				{snapshot: snapshot(110, 115, SourceIdentity{Instance: "node", Epoch: "one"}, flow("r", "vless/reality", 60, 65, started))},
				{snapshot: snapshot(2, 3, SourceIdentity{Instance: "node", Epoch: "one"}, flow("r2", "vless/reality", 2, 3, baseTestTime.Add(2*time.Second)))},
				{snapshot: snapshot(6, 8, SourceIdentity{Instance: "node", Epoch: "one"}, flow("r2", "vless/reality", 6, 8, baseTestTime.Add(2*time.Second)))},
			},
		},
		{
			name: "epoch change",
			results: []scriptedResult{
				{snapshot: snapshot(100, 100, SourceIdentity{Instance: "node", Epoch: "one"}, flow("r", "vless/reality", 50, 50, started))},
				{snapshot: snapshot(110, 115, SourceIdentity{Instance: "node", Epoch: "one"}, flow("r", "vless/reality", 60, 65, started))},
				{snapshot: snapshot(1000, 1000, SourceIdentity{Instance: "node", Epoch: "two"}, flow("h", "hysteria2/hysteria2", 500, 500, baseTestTime.Add(2*time.Second)))},
				{snapshot: snapshot(1004, 1005, SourceIdentity{Instance: "node", Epoch: "two"}, flow("h", "hysteria2/hysteria2", 504, 505, baseTestTime.Add(2*time.Second)))},
			},
		},
		{
			name: "instance change",
			results: []scriptedResult{
				{snapshot: snapshot(100, 100, SourceIdentity{Instance: "one", Epoch: "epoch"}, flow("r", "vless/reality", 50, 50, started))},
				{snapshot: snapshot(110, 115, SourceIdentity{Instance: "one", Epoch: "epoch"}, flow("r", "vless/reality", 60, 65, started))},
				{snapshot: snapshot(1000, 1000, SourceIdentity{Instance: "two", Epoch: "epoch"}, flow("h", "hysteria2/hysteria2", 500, 500, baseTestTime.Add(2*time.Second)))},
				{snapshot: snapshot(1004, 1005, SourceIdentity{Instance: "two", Epoch: "epoch"}, flow("h", "hysteria2/hysteria2", 504, 505, baseTestTime.Add(2*time.Second)))},
			},
		},
		{
			name: "identity becomes available",
			results: []scriptedResult{
				{snapshot: snapshot(100, 100, SourceIdentity{}, flow("r", "vless/reality", 50, 50, started))},
				{snapshot: snapshot(110, 115, SourceIdentity{}, flow("r", "vless/reality", 60, 65, started))},
				{snapshot: snapshot(1000, 1000, SourceIdentity{Instance: "node", Epoch: "known"}, flow("h", "hysteria2/hysteria2", 500, 500, baseTestTime.Add(2*time.Second)))},
				{snapshot: snapshot(1004, 1005, SourceIdentity{Instance: "node", Epoch: "known"}, flow("h", "hysteria2/hysteria2", 504, 505, baseTestTime.Add(2*time.Second)))},
			},
		},
		{
			name: "identity becomes unavailable",
			results: []scriptedResult{
				{snapshot: snapshot(100, 100, SourceIdentity{Instance: "node", Epoch: "known"}, flow("r", "vless/reality", 50, 50, started))},
				{snapshot: snapshot(110, 115, SourceIdentity{Instance: "node", Epoch: "known"}, flow("r", "vless/reality", 60, 65, started))},
				{snapshot: snapshot(1000, 1000, SourceIdentity{}, flow("h", "hysteria2/hysteria2", 500, 500, baseTestTime.Add(2*time.Second)))},
				{snapshot: snapshot(1004, 1005, SourceIdentity{}, flow("h", "hysteria2/hysteria2", 504, 505, baseTestTime.Add(2*time.Second)))},
			},
		},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			now := baseTestTime
			collector := newTestCollector(t, &scriptedSource{results: test.results}, nil, &now)
			collectAt(collector, &now, baseTestTime)
			beforeReset := collectAt(collector, &now, baseTestTime.Add(time.Second))
			if beforeReset.Traffic.Reality != (ByteCounters{Upload: 10, Download: 15}) {
				t.Fatalf("unexpected traffic before reset: %#v", beforeReset.Traffic)
			}
			atReset := collectAt(collector, &now, baseTestTime.Add(2*time.Second))
			if atReset.Traffic != beforeReset.Traffic {
				t.Fatalf("reset sample added traffic: before %#v after %#v", beforeReset.Traffic, atReset.Traffic)
			}
			afterReset := collectAt(collector, &now, baseTestTime.Add(3*time.Second))
			if afterReset.Total.Upload != 14 || afterReset.Total.Download != 20 {
				t.Fatalf("unexpected post-reset totals: %#v", afterReset)
			}
		})
	}
}

func TestCollectorAheadCarry(t *testing.T) {
	now := baseTestTime
	started := baseTestTime.Add(-time.Minute)
	source := &scriptedSource{results: []scriptedResult{
		{snapshot: snapshot(0, 0, SourceIdentity{}, flow("r", "vless/reality", 0, 0, started))},
		{snapshot: snapshot(5, 5, SourceIdentity{}, flow("r", "vless/reality", 8, 9, started))},
		{snapshot: snapshot(8, 9, SourceIdentity{}, flow("r", "vless/reality", 8, 9, started))},
		{snapshot: snapshot(10, 12, SourceIdentity{}, flow("r", "vless/reality", 10, 12, started))},
		{snapshot: snapshot(14, 16, SourceIdentity{}, flow("r", "vless/reality", 14, 16, started))},
	}}
	collector := newTestCollector(t, source, nil, &now)
	collectAt(collector, &now, baseTestTime)
	first := collectAt(collector, &now, baseTestTime.Add(time.Second))
	if first.Traffic.Reality != (ByteCounters{Upload: 5, Download: 5}) || first.Traffic.Unattributed != (ByteCounters{}) {
		t.Fatalf("known delta should be capped by global: %#v", first.Traffic)
	}
	second := collectAt(collector, &now, baseTestTime.Add(2*time.Second))
	if second.Traffic.Reality != (ByteCounters{Upload: 8, Download: 9}) || second.Traffic.Unattributed != (ByteCounters{}) {
		t.Fatalf("ahead carry should consume later global delta: %#v", second.Traffic)
	}
	third := collectAt(collector, &now, baseTestTime.Add(3*time.Second))
	if third.Traffic.Reality != (ByteCounters{Upload: 10, Download: 12}) || third.Traffic.Unattributed != (ByteCounters{}) {
		t.Fatalf("new observed bytes were double-counted after ahead carry: %#v", third.Traffic)
	}
	fourth := collectAt(collector, &now, baseTestTime.Add(4*time.Second))
	if fourth.Traffic.Reality != (ByteCounters{Upload: 14, Download: 16}) || fourth.Traffic.Unattributed != (ByteCounters{}) {
		t.Fatalf("continued traffic was lost while carry existed: %#v", fourth.Traffic)
	}
}

func TestCollectorAPIRecovery(t *testing.T) {
	now := baseTestTime
	started := baseTestTime.Add(-time.Minute)
	source := &scriptedSource{results: []scriptedResult{
		{snapshot: snapshot(0, 0, SourceIdentity{}, flow("r", "vless/reality", 0, 0, started))},
		{err: errors.New("API unavailable")},
		{snapshot: snapshot(12, 15, SourceIdentity{}, flow("r", "vless/reality", 7, 9, started))},
	}}
	collector := newTestCollector(t, source, nil, &now)
	collectAt(collector, &now, baseTestTime)
	failed := collectAt(collector, &now, baseTestTime.Add(time.Second))
	if failed.Available || !failed.Stale || failed.Traffic.Total() != (ByteCounters{}) {
		t.Fatalf("unexpected unavailable view: %#v", failed)
	}
	recovered := collectAt(collector, &now, baseTestTime.Add(2*time.Second))
	want := ProtocolCounters{Reality: ByteCounters{Upload: 7, Download: 9}, Unattributed: ByteCounters{Upload: 5, Download: 6}}
	if !recovered.Available || recovered.Stale || recovered.Traffic != want {
		t.Fatalf("unexpected recovery attribution: got %#v want %#v", recovered, want)
	}
}

func TestCollectorFirstFailureDoesNotInventHistory(t *testing.T) {
	now := baseTestTime
	collector := newTestCollector(t, &scriptedSource{results: []scriptedResult{{err: errors.New("down")}}}, nil, &now)
	view := collectAt(collector, &now, baseTestTime)
	if view.Available || !view.Stale {
		t.Fatalf("unexpected failed view: %#v", view)
	}
	history, err := collector.History(History1Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 0 {
		t.Fatalf("first failed request invented history: %#v", history)
	}
}

func TestCollectorOutageMarksEveryElapsedMinute(t *testing.T) {
	now := baseTestTime
	source := &scriptedSource{results: []scriptedResult{
		{snapshot: snapshot(0, 0, SourceIdentity{})},
		{err: errors.New("down")},
	}}
	collector := newTestCollector(t, source, nil, &now)
	collectAt(collector, &now, baseTestTime)
	collectAt(collector, &now, baseTestTime.Add(3*time.Minute+5*time.Second))
	history, err := collector.History(History1Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 4 {
		t.Fatalf("gap points = %d, want 4: %#v", len(history), history)
	}
	for index, point := range history {
		wantMinute := baseTestTime.Add(time.Duration(index) * time.Minute)
		if !point.Minute.Equal(wantMinute) || !point.Gap {
			t.Fatalf("gap point %d = %#v, want minute %v", index, point, wantMinute)
		}
	}
}

func TestCollectorSnapshotBecomesStaleWithClock(t *testing.T) {
	now := baseTestTime
	collector, err := NewCollector(&scriptedSource{results: []scriptedResult{{snapshot: snapshot(0, 0, SourceIdentity{})}}}, nil,
		WithClock(func() time.Time { return now }), WithStaleAfter(2*time.Second))
	if err != nil {
		t.Fatal(err)
	}
	collector.collect(context.Background(), now)
	if collector.Snapshot().Stale {
		t.Fatal("fresh sample reported stale")
	}
	now = now.Add(2 * time.Second)
	if !collector.Snapshot().Stale {
		t.Fatal("old sample did not become stale")
	}
}

func TestCollectorPersistentRecoveryDoesNotDoubleCount(t *testing.T) {
	now := baseTestTime
	started := baseTestTime.Add(-time.Minute)
	store := &memoryStateStore{}
	firstSource := &scriptedSource{results: []scriptedResult{
		{snapshot: snapshot(100, 200, SourceIdentity{Instance: "node", Epoch: "epoch"}, flow("r", "vless/reality", 40, 80, started))},
		{snapshot: snapshot(110, 220, SourceIdentity{Instance: "node", Epoch: "epoch"}, flow("r", "vless/reality", 50, 100, started))},
	}}
	first := newTestCollector(t, firstSource, store, &now)
	collectAt(first, &now, baseTestTime)
	before := collectAt(first, &now, baseTestTime.Add(time.Second))
	if before.Traffic.Reality != (ByteCounters{Upload: 10, Download: 20}) {
		t.Fatalf("unexpected pre-checkpoint traffic: %#v", before.Traffic)
	}
	if err := first.Flush(); err != nil {
		t.Fatal(err)
	}

	secondSource := &scriptedSource{results: []scriptedResult{
		{snapshot: snapshot(110, 220, SourceIdentity{Instance: "node", Epoch: "epoch"}, flow("r", "vless/reality", 50, 100, started))},
		{snapshot: snapshot(115, 227, SourceIdentity{Instance: "node", Epoch: "epoch"}, flow("r", "vless/reality", 55, 107, started))},
	}}
	second := newTestCollector(t, secondSource, store, &now)
	resumed := collectAt(second, &now, baseTestTime.Add(2*time.Second))
	if resumed.Traffic != before.Traffic {
		t.Fatalf("identical resume snapshot double-counted: before %#v after %#v", before.Traffic, resumed.Traffic)
	}
	after := collectAt(second, &now, baseTestTime.Add(3*time.Second))
	if after.Traffic.Reality != (ByteCounters{Upload: 15, Download: 27}) {
		t.Fatalf("unexpected traffic after resume: %#v", after.Traffic)
	}
}

func TestCollectorPersistentRecoveryWithMissingFlowRebaselines(t *testing.T) {
	now := baseTestTime
	state := newPersistentState()
	state.Initialized = true
	state.Source = SourceIdentity{Instance: "node", Epoch: "epoch"}
	state.Global = ByteCounters{Upload: 100, Download: 100}
	state.Traffic.Reality = ByteCounters{Upload: 10, Download: 10}
	state.Open["ended"] = OpenFlow{Protocol: ProtocolReality, Upload: 50, Download: 50, Start: baseTestTime.Add(-time.Minute)}
	store := &memoryStateStore{state: state, has: true}
	source := &scriptedSource{results: []scriptedResult{
		{snapshot: snapshot(120, 130, state.Source)},
		{snapshot: snapshot(124, 135, state.Source)},
	}}
	collector := newTestCollector(t, source, store, &now)
	baseline := collectAt(collector, &now, baseTestTime)
	if baseline.Traffic.Reality != (ByteCounters{Upload: 10, Download: 10}) || baseline.Traffic.Unattributed != (ByteCounters{}) {
		t.Fatalf("missing persisted flow should force a safe baseline: %#v", baseline.Traffic)
	}
	after := collectAt(collector, &now, baseTestTime.Add(time.Second))
	if after.Traffic.Unattributed != (ByteCounters{Upload: 4, Download: 5}) {
		t.Fatalf("new post-baseline traffic was not counted: %#v", after.Traffic)
	}
}

func TestCollectorPersistentRecoveryRejectsReusedFlowID(t *testing.T) {
	now := baseTestTime
	started := baseTestTime.Add(-time.Minute)
	state := newPersistentState()
	state.Initialized = true
	state.Source = SourceIdentity{Instance: "node", Epoch: "epoch"}
	state.Global = ByteCounters{Upload: 100, Download: 100}
	state.Traffic.Reality = ByteCounters{Upload: 10, Download: 10}
	state.Open["same-id"] = OpenFlow{Protocol: ProtocolReality, Upload: 50, Download: 50, Start: started}
	store := &memoryStateStore{state: state, has: true}
	source := &scriptedSource{results: []scriptedResult{
		{snapshot: snapshot(1000, 1000, state.Source, flow("same-id", "vless/reality", 500, 500, baseTestTime))},
		{snapshot: snapshot(1004, 1005, state.Source, flow("same-id", "vless/reality", 504, 505, baseTestTime))},
	}}
	collector := newTestCollector(t, source, store, &now)
	baseline := collectAt(collector, &now, baseTestTime)
	if baseline.Traffic.Reality != (ByteCounters{Upload: 10, Download: 10}) {
		t.Fatalf("reused flow id should establish baseline: %#v", baseline.Traffic)
	}
	after := collectAt(collector, &now, baseTestTime.Add(time.Second))
	if after.Traffic.Reality != (ByteCounters{Upload: 14, Download: 15}) {
		t.Fatalf("unexpected traffic after fresh baseline: %#v", after.Traffic)
	}
}

func TestCollectorHistoryBucketsAndRetention(t *testing.T) {
	now := baseTestTime
	source := &scriptedSource{results: []scriptedResult{{snapshot: snapshot(0, 0, SourceIdentity{})}}}
	collector := newTestCollector(t, source, nil, &now)
	collector.collect(context.Background(), now)

	collector.mu.Lock()
	collector.state.History = make([]MinutePoint, MaxHistoryPoints)
	start := baseTestTime.Add(-(MaxHistoryPoints - 1) * time.Minute)
	for index := range collector.state.History {
		minute := start.Add(time.Duration(index) * time.Minute)
		delta := ProtocolCounters{Reality: ByteCounters{Upload: uint64(index + 1)}}
		collector.state.History[index] = MinutePoint{Minute: minute, Traffic: delta, Total: delta.Total()}
	}
	collector.addHistory(baseTestTime.Add(time.Minute), ProtocolCounters{Hysteria2: ByteCounters{Download: 7}}, false)
	if len(collector.state.History) != MaxHistoryPoints {
		collector.mu.Unlock()
		t.Fatalf("history length = %d, want %d", len(collector.state.History), MaxHistoryPoints)
	}
	if !collector.state.History[0].Minute.Equal(start.Add(time.Minute)) {
		collector.mu.Unlock()
		t.Fatalf("unexpected first retained point: %v", collector.state.History[0].Minute)
	}
	collector.mu.Unlock()

	now = baseTestTime.Add(time.Minute)
	history, err := collector.History(History1Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 61 {
		t.Fatalf("1h history points = %d, want 61 minute timestamps including cutoff", len(history))
	}
	if _, err := collector.History(HistoryRange("30d")); err == nil {
		t.Fatal("expected invalid range to fail")
	}
}

func TestCollectorHistoryAggregatesMinuteAndMarksGap(t *testing.T) {
	now := baseTestTime
	started := baseTestTime.Add(-time.Minute)
	source := &scriptedSource{results: []scriptedResult{
		{snapshot: snapshot(0, 0, SourceIdentity{}, flow("r", "vless/reality", 0, 0, started))},
		{snapshot: snapshot(3, 4, SourceIdentity{}, flow("r", "vless/reality", 3, 4, started))},
		{err: errors.New("down")},
		{snapshot: snapshot(8, 10, SourceIdentity{}, flow("r", "vless/reality", 8, 10, started))},
	}}
	collector := newTestCollector(t, source, nil, &now)
	collectAt(collector, &now, baseTestTime)
	collectAt(collector, &now, baseTestTime.Add(10*time.Second))
	collectAt(collector, &now, baseTestTime.Add(20*time.Second))
	collectAt(collector, &now, baseTestTime.Add(61*time.Second))
	history, err := collector.History(History1Hour)
	if err != nil {
		t.Fatal(err)
	}
	if len(history) != 2 {
		t.Fatalf("history = %#v", history)
	}
	if history[0].Traffic.Reality != (ByteCounters{Upload: 3, Download: 4}) || !history[0].Gap {
		t.Fatalf("unexpected first minute: %#v", history[0])
	}
	if history[1].Traffic.Reality != (ByteCounters{Upload: 5, Download: 6}) {
		t.Fatalf("unexpected second minute: %#v", history[1])
	}
}

type advancingSource struct {
	mu      sync.Mutex
	clockMu *sync.Mutex
	now     *time.Time
	step    time.Duration
	fetches int
}

func (s *advancingSource) Fetch(context.Context) (ClashSnapshot, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clockMu.Lock()
	*s.now = s.now.Add(s.step)
	s.clockMu.Unlock()
	s.fetches++
	return snapshot(uint64(s.fetches), uint64(s.fetches), SourceIdentity{}), nil
}

func TestCollectorRunCheckpointsCompletedMinutes(t *testing.T) {
	var nowMu sync.Mutex
	now := baseTestTime
	clock := func() time.Time {
		nowMu.Lock()
		defer nowMu.Unlock()
		return now
	}
	store := &memoryStateStore{}
	source := &advancingSource{clockMu: &nowMu, now: &now, step: time.Minute}
	collector, err := NewCollector(source, store, WithSampleInterval(time.Millisecond), WithClock(clock))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- collector.Run(ctx) }()
	deadline := time.Now().Add(time.Second)
	for {
		store.mu.Lock()
		saves := store.saves
		store.mu.Unlock()
		if saves > 0 {
			break
		}
		if time.Now().After(deadline) {
			cancel()
			t.Fatal("Run did not checkpoint at a minute boundary")
		}
		time.Sleep(time.Millisecond)
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestCollectorRunFlushAndSingleWriter(t *testing.T) {
	var nowMu sync.Mutex
	now := baseTestTime
	clock := func() time.Time {
		nowMu.Lock()
		defer nowMu.Unlock()
		now = now.Add(time.Millisecond)
		return now
	}
	store := &memoryStateStore{}
	source := &scriptedSource{results: []scriptedResult{{snapshot: snapshot(0, 0, SourceIdentity{})}}}
	collector, err := NewCollector(source, store, WithSampleInterval(time.Millisecond), WithClock(clock))
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan error, 1)
	go func() { done <- collector.Run(ctx) }()
	time.Sleep(10 * time.Millisecond)
	if err := collector.Run(context.Background()); err == nil {
		t.Fatal("expected concurrent Run to fail")
	}
	cancel()
	if err := <-done; err != nil {
		t.Fatal(err)
	}
	store.mu.Lock()
	saves := store.saves
	store.mu.Unlock()
	if saves == 0 {
		t.Fatal("Run did not flush state on shutdown")
	}
}

func TestCollectorConcurrentPublicAPI(t *testing.T) {
	now := baseTestTime
	store := &memoryStateStore{}
	collector := newTestCollector(t, &scriptedSource{results: []scriptedResult{{snapshot: snapshot(0, 0, SourceIdentity{})}}}, store, &now)
	collector.collect(context.Background(), now)
	var wait sync.WaitGroup
	for worker := 0; worker < 16; worker++ {
		wait.Add(1)
		go func() {
			defer wait.Done()
			for iteration := 0; iteration < 100; iteration++ {
				_ = collector.Snapshot()
				_, _ = collector.History(History1Hour)
				_ = collector.Checkpoint()
			}
		}()
	}
	wait.Wait()
}

func TestReconcileDirectionTable(t *testing.T) {
	tests := []struct {
		name                                        string
		global, reality, hysteria, unknown          uint64
		realityAhead, hysteriaAhead                 uint64
		wantReality, wantHysteria, wantUnattributed uint64
		wantRealityAhead, wantHysteriaAhead         uint64
	}{
		{name: "exact", global: 10, reality: 4, hysteria: 6, wantReality: 4, wantHysteria: 6},
		{name: "short gap", global: 15, reality: 4, hysteria: 6, wantReality: 4, wantHysteria: 6, wantUnattributed: 5},
		{name: "known ahead", global: 5, reality: 8, wantReality: 5, wantRealityAhead: 3},
		{name: "carry catches global", global: 3, realityAhead: 3, wantReality: 3},
		{name: "observed while prior carry clears", global: 5, reality: 3, realityAhead: 3, wantReality: 5, wantRealityAhead: 1},
		{name: "unknown consumes global", global: 7, unknown: 5, wantUnattributed: 7},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			r, h, u, ra, ha := reconcileDirection(test.global, test.reality, test.hysteria, test.unknown, test.realityAhead, test.hysteriaAhead)
			got := fmt.Sprint(r, h, u, ra, ha)
			want := fmt.Sprint(test.wantReality, test.wantHysteria, test.wantUnattributed, test.wantRealityAhead, test.wantHysteriaAhead)
			if got != want {
				t.Fatalf("got %s, want %s", got, want)
			}
		})
	}
}
