package traffic

import (
	"context"
	"errors"
	"sort"
	"sync"
	"time"
)

const (
	defaultSampleInterval = time.Second
	defaultStaleAfter     = 3 * time.Second
)

// Source supplies one privacy-minimal Clash traffic snapshot.
type Source interface {
	Fetch(context.Context) (ClashSnapshot, error)
}

// StateStore is the durable checkpoint boundary used by Collector.
type StateStore interface {
	Load() (PersistentState, error)
	Save(PersistentState) error
}

// CollectorOption configures integration and deterministic tests.
type CollectorOption func(*collectorOptions) error

type collectorOptions struct {
	interval   time.Duration
	staleAfter time.Duration
	now        func() time.Time
}

// WithSampleInterval changes the Run ticker interval. The normal production
// value is one second.
func WithSampleInterval(interval time.Duration) CollectorOption {
	return func(options *collectorOptions) error {
		if interval <= 0 {
			return errors.New("traffic sample interval must be positive")
		}
		options.interval = interval
		return nil
	}
}

// WithStaleAfter changes how long the last successful sample remains fresh.
func WithStaleAfter(duration time.Duration) CollectorOption {
	return func(options *collectorOptions) error {
		if duration <= 0 {
			return errors.New("traffic stale duration must be positive")
		}
		options.staleAfter = duration
		return nil
	}
}

// WithClock injects the wall clock used for samples, history, and checkpoints.
func WithClock(now func() time.Time) CollectorOption {
	return func(options *collectorOptions) error {
		if now == nil {
			return errors.New("traffic clock is nil")
		}
		options.now = now
		return nil
	}
}

// Collector has a single mutation lock. Run, Snapshot, History, Checkpoint,
// Flush, and direct collection in tests may execute concurrently without races.
type Collector struct {
	mu sync.Mutex

	source Source
	store  StateStore

	interval   time.Duration
	staleAfter time.Duration
	now        func() time.Time

	state       PersistentState
	live        LiveSnapshot
	dirty       bool
	lastAttempt time.Time
	lastRateAt  time.Time
	needsResume bool

	runMu   sync.Mutex
	running bool
}

// NewCollector loads any existing checkpoint and prepares a collector. No
// network request is made until Run starts (or collect is exercised by tests).
func NewCollector(source Source, store StateStore, options ...CollectorOption) (*Collector, error) {
	if source == nil {
		return nil, errors.New("traffic source is nil")
	}
	configured := collectorOptions{
		interval:   defaultSampleInterval,
		staleAfter: defaultStaleAfter,
		now:        time.Now,
	}
	for _, option := range options {
		if option == nil {
			continue
		}
		if err := option(&configured); err != nil {
			return nil, err
		}
	}
	state := newPersistentState()
	if store != nil {
		loaded, err := store.Load()
		if err != nil {
			return nil, err
		}
		state = loaded
	}
	if state.Open == nil {
		state.Open = make(map[string]OpenFlow)
	}
	collector := &Collector{
		source:      source,
		store:       store,
		interval:    configured.interval,
		staleAfter:  configured.staleAfter,
		now:         configured.now,
		state:       state,
		needsResume: state.Initialized,
	}
	collector.live = LiveSnapshot{
		Traffic: state.Traffic,
		Total:   state.Traffic.Total(),
		Stale:   true,
	}
	return collector, nil
}

// Run samples immediately, then at the configured interval until ctx ends. A
// final Flush is attempted on shutdown. Only one Run may be active.
func (c *Collector) Run(ctx context.Context) error {
	if c == nil {
		return errors.New("traffic collector is nil")
	}
	if ctx == nil {
		return errors.New("traffic collector context is nil")
	}
	c.runMu.Lock()
	if c.running {
		c.runMu.Unlock()
		return errors.New("traffic collector is already running")
	}
	c.running = true
	c.runMu.Unlock()
	defer func() {
		c.runMu.Lock()
		c.running = false
		c.runMu.Unlock()
	}()

	c.collect(ctx, c.now().UTC())
	checkpointMinute := c.latestAttemptMinute()
	ticker := time.NewTicker(c.interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			if err := c.Flush(); err != nil {
				return err
			}
			return nil
		case <-ticker.C:
			c.collect(ctx, c.now().UTC())
			minute := c.latestAttemptMinute()
			if !checkpointMinute.IsZero() && minute.After(checkpointMinute) {
				if err := c.Checkpoint(); err != nil {
					return err
				}
			}
			checkpointMinute = minute
		}
	}
}

func (c *Collector) latestAttemptMinute() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.lastAttempt.IsZero() {
		return time.Time{}
	}
	return c.lastAttempt.UTC().Truncate(time.Minute)
}

// Snapshot returns a value copy of the latest live view and evaluates staleness
// at call time, so an API outage is visible even between sampling callbacks.
func (c *Collector) Snapshot() LiveSnapshot {
	if c == nil {
		return LiveSnapshot{Stale: true}
	}
	now := c.now().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	result := c.live
	if result.LastSuccess.IsZero() || now.Sub(result.LastSuccess) >= c.staleAfter {
		result.Stale = true
	}
	return result
}

// History returns chronological points whose minute lies in the selected range.
func (c *Collector) History(historyRange HistoryRange) ([]MinutePoint, error) {
	if c == nil {
		return nil, errors.New("traffic collector is nil")
	}
	duration, err := historyRange.duration()
	if err != nil {
		return nil, err
	}
	now := c.now().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	cutoff := now.Add(-duration).Truncate(time.Minute)
	index := sort.Search(len(c.state.History), func(index int) bool {
		return !c.state.History[index].Minute.Before(cutoff)
	})
	result := make([]MinutePoint, len(c.state.History)-index)
	copy(result, c.state.History[index:])
	return result, nil
}

// Checkpoint persists a durable copy without changing collection semantics.
// It is intentionally serialized with collection so a successful return always
// means the current in-memory state was durably saved.
func (c *Collector) Checkpoint() error {
	return c.checkpoint(false)
}

// Flush forces a checkpoint, including when no sample changed counters.
func (c *Collector) Flush() error {
	return c.checkpoint(true)
}

func (c *Collector) checkpoint(force bool) error {
	if c == nil {
		return errors.New("traffic collector is nil")
	}
	if c.store == nil {
		return nil
	}
	savedAt := c.now().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	if !force && !c.dirty {
		return nil
	}
	state := clonePersistentState(c.state)
	state.SavedAt = savedAt
	if err := c.store.Save(state); err != nil {
		return err
	}
	c.state.SavedAt = state.SavedAt
	c.dirty = false
	return nil
}

func (c *Collector) collect(ctx context.Context, sampledAt time.Time) {
	if sampledAt.IsZero() {
		sampledAt = c.now().UTC()
	}
	snapshot, err := c.source.Fetch(ctx)
	finishedAt := c.now().UTC()
	c.mu.Lock()
	defer c.mu.Unlock()
	previousAttempt := c.lastAttempt
	if finishedAt.After(sampledAt) {
		sampledAt = finishedAt
	}
	c.lastAttempt = sampledAt
	if err != nil {
		c.markUnavailable(sampledAt, previousAttempt)
		return
	}
	if validationErr := validateClashSnapshot(snapshot); validationErr != nil {
		c.markUnavailable(sampledAt, previousAttempt)
		return
	}
	if !previousAttempt.IsZero() && sampledAt.After(previousAttempt.Add(c.interval*2)) {
		c.markGapRange(previousAttempt.Add(c.interval), sampledAt.Add(-c.interval))
	}

	if !c.state.Initialized {
		c.establishBaseline(snapshot, sampledAt)
		return
	}
	if countersDecreased(c.state.Global, snapshot) {
		c.resetBaseline(snapshot, sampledAt)
		return
	}
	if sourceChanged(c.state.Source, snapshot.Source) {
		c.resetBaseline(snapshot, sampledAt)
		return
	}
	if c.needsResume {
		if c.canResume(snapshot) {
			c.needsResume = false
			c.lastRateAt = sampledAt
			c.applySnapshot(snapshot, sampledAt)
		} else {
			c.resetBaseline(snapshot, sampledAt)
		}
		return
	}
	c.applySnapshot(snapshot, sampledAt)
}

func (c *Collector) establishBaseline(snapshot ClashSnapshot, sampledAt time.Time) {
	c.state.Initialized = true
	c.state.Source = snapshot.Source
	c.state.Global = ByteCounters{Upload: snapshot.UploadTotal, Download: snapshot.DownloadTotal}
	c.state.Open = snapshotOpenFlows(snapshot)
	c.state.Ahead = ProtocolCounters{}
	c.needsResume = false
	c.updateLive(sampledAt, ProtocolCounters{}, sampleInterval(c.lastRateAt, sampledAt))
	c.lastRateAt = sampledAt
	c.dirty = true
}

func (c *Collector) resetBaseline(snapshot ClashSnapshot, sampledAt time.Time) {
	c.state.Source = snapshot.Source
	c.state.Global = ByteCounters{Upload: snapshot.UploadTotal, Download: snapshot.DownloadTotal}
	c.state.Open = snapshotOpenFlows(snapshot)
	c.state.Ahead = ProtocolCounters{}
	c.needsResume = false
	c.updateLive(sampledAt, ProtocolCounters{}, sampleInterval(c.lastRateAt, sampledAt))
	c.lastRateAt = sampledAt
	c.dirty = true
}

func (c *Collector) canResume(snapshot ClashSnapshot) bool {
	for id, old := range c.state.Open {
		connection, found := findConnection(snapshot.Connections, id)
		if !found || !sameFlow(old, connection) || connection.Upload < old.Upload || connection.Download < old.Download {
			return false
		}
	}
	return true
}

func (c *Collector) applySnapshot(snapshot ClashSnapshot, sampledAt time.Time) {
	globalDelta := ByteCounters{
		Upload:   snapshot.UploadTotal - c.state.Global.Upload,
		Download: snapshot.DownloadTotal - c.state.Global.Download,
	}
	observed := observedConnectionDeltas(c.state.Open, snapshot)
	allocated, ahead := reconcile(globalDelta, observed, c.state.Ahead)
	for _, protocol := range protocolOrder {
		counter := c.state.Traffic.counter(protocol)
		delta := allocated.For(protocol)
		counter.Upload = saturatingSum(counter.Upload, delta.Upload)
		counter.Download = saturatingSum(counter.Download, delta.Download)
	}
	c.state.Ahead = ahead
	c.state.Global = ByteCounters{Upload: snapshot.UploadTotal, Download: snapshot.DownloadTotal}
	c.state.Open = snapshotOpenFlows(snapshot)
	c.addHistory(sampledAt, allocated, false)
	interval := sampleInterval(c.lastRateAt, sampledAt)
	c.updateLive(sampledAt, allocated, interval)
	c.lastRateAt = sampledAt
	c.dirty = true
}

func (c *Collector) markUnavailable(sampledAt, previousAttempt time.Time) {
	c.live.SampledAt = sampledAt
	c.live.Available = false
	c.live.Stale = true
	c.live.Rates = ProtocolRates{}
	c.live.ActiveFlows = FlowCounts{}
	if previousAttempt.IsZero() {
		return
	}
	c.markGapRange(previousAttempt.Add(c.interval), sampledAt)
	c.dirty = true
}

func (c *Collector) updateLive(sampledAt time.Time, delta ProtocolCounters, interval time.Duration) {
	c.live.SampledAt = sampledAt
	c.live.LastSuccess = sampledAt
	c.live.Available = true
	c.live.Stale = false
	c.live.Traffic = c.state.Traffic
	c.live.Total = c.state.Traffic.Total()
	c.live.ActiveFlows = activeFlowCounts(c.state.Open)
	c.live.Rates = rates(delta, interval)
}

func (c *Collector) addHistory(sampledAt time.Time, delta ProtocolCounters, gap bool) {
	minute := sampledAt.UTC().Truncate(time.Minute)
	if len(c.state.History) > 0 && c.state.History[len(c.state.History)-1].Minute.Equal(minute) {
		point := &c.state.History[len(c.state.History)-1]
		mergeCounters(&point.Traffic, delta)
		point.Total = point.Traffic.Total()
		point.Gap = point.Gap || gap
	} else {
		c.state.History = append(c.state.History, MinutePoint{
			Minute:  minute,
			Traffic: delta,
			Total:   delta.Total(),
			Gap:     gap,
		})
	}
	c.pruneHistory(sampledAt)
}

func (c *Collector) markGapRange(firstMissing, sampledAt time.Time) {
	minute := firstMissing.UTC().Truncate(time.Minute)
	last := sampledAt.UTC().Truncate(time.Minute)
	for !minute.After(last) {
		c.markMinuteGap(minute)
		minute = minute.Add(time.Minute)
	}
}

func (c *Collector) markMinuteGap(sampledAt time.Time) {
	minute := sampledAt.UTC().Truncate(time.Minute)
	if len(c.state.History) > 0 && c.state.History[len(c.state.History)-1].Minute.Equal(minute) {
		c.state.History[len(c.state.History)-1].Gap = true
		return
	}
	c.state.History = append(c.state.History, MinutePoint{Minute: minute, Gap: true})
	c.pruneHistory(sampledAt)
}

func (c *Collector) pruneHistory(now time.Time) {
	cutoff := now.UTC().Add(-7 * 24 * time.Hour).Truncate(time.Minute)
	first := sort.Search(len(c.state.History), func(index int) bool {
		return !c.state.History[index].Minute.Before(cutoff)
	})
	if first > 0 {
		copy(c.state.History, c.state.History[first:])
		c.state.History = c.state.History[:len(c.state.History)-first]
	}
	if len(c.state.History) > MaxHistoryPoints {
		drop := len(c.state.History) - MaxHistoryPoints
		copy(c.state.History, c.state.History[drop:])
		c.state.History = c.state.History[:MaxHistoryPoints]
	}
}

func validateClashSnapshot(snapshot ClashSnapshot) error {
	if !validSourceIdentity(snapshot.Source) {
		return errors.New("invalid Clash source identity")
	}
	if len(snapshot.Connections) > MaxOpenFlows {
		return errors.New("too many Clash connections")
	}
	seen := make(map[string]struct{}, len(snapshot.Connections))
	for _, connection := range snapshot.Connections {
		if connection.ID == "" || len(connection.ID) > 256 || connection.Metadata.Type == "" || connection.Start.IsZero() {
			return errors.New("invalid Clash connection")
		}
		if _, exists := seen[connection.ID]; exists {
			return errors.New("duplicate Clash connection id")
		}
		seen[connection.ID] = struct{}{}
	}
	return nil
}

func observedConnectionDeltas(previous map[string]OpenFlow, snapshot ClashSnapshot) ProtocolCounters {
	var observed ProtocolCounters
	for _, connection := range snapshot.Connections {
		protocol := classify(connection.Metadata.Type)
		old, found := previous[connection.ID]
		if found && sameFlow(old, connection) {
			delta := ByteCounters{}
			if connection.Upload >= old.Upload {
				delta.Upload = connection.Upload - old.Upload
			} else {
				delta.Upload = connection.Upload
			}
			if connection.Download >= old.Download {
				delta.Download = connection.Download - old.Download
			} else {
				delta.Download = connection.Download
			}
			mergeBytes(observed.counter(protocol), delta)
			continue
		}
		mergeBytes(observed.counter(protocol), ByteCounters{Upload: connection.Upload, Download: connection.Download})
	}
	return observed
}

func reconcile(global ByteCounters, observed ProtocolCounters, previousAhead ProtocolCounters) (ProtocolCounters, ProtocolCounters) {
	var allocated ProtocolCounters
	var nextAhead ProtocolCounters
	allocated.Reality.Upload, allocated.Hysteria2.Upload, allocated.Unattributed.Upload, nextAhead.Reality.Upload, nextAhead.Hysteria2.Upload = reconcileDirection(
		global.Upload,
		observed.Reality.Upload,
		observed.Hysteria2.Upload,
		observed.Unattributed.Upload,
		previousAhead.Reality.Upload,
		previousAhead.Hysteria2.Upload,
	)
	allocated.Reality.Download, allocated.Hysteria2.Download, allocated.Unattributed.Download, nextAhead.Reality.Download, nextAhead.Hysteria2.Download = reconcileDirection(
		global.Download,
		observed.Reality.Download,
		observed.Hysteria2.Download,
		observed.Unattributed.Download,
		previousAhead.Reality.Download,
		previousAhead.Hysteria2.Download,
	)
	return allocated, nextAhead
}

func reconcileDirection(global, realityObserved, hysteriaObserved, unknownObserved, realityAhead, hysteriaAhead uint64) (uint64, uint64, uint64, uint64, uint64) {
	available := global

	knownAhead := saturatingSum(realityAhead, hysteriaAhead)
	consumeAhead := minUint64(available, knownAhead)
	realityCarry := minUint64(realityAhead, consumeAhead)
	remainingCarry := consumeAhead - realityCarry
	hysteriaCarry := minUint64(hysteriaAhead, remainingCarry)
	available -= realityCarry + hysteriaCarry
	realityAhead -= realityCarry
	hysteriaAhead -= hysteriaCarry

	realityAllocate := minUint64(realityObserved, available)
	available -= realityAllocate
	realityExcess := realityObserved - realityAllocate
	hysteriaAllocate := minUint64(hysteriaObserved, available)
	available -= hysteriaAllocate
	hysteriaExcess := hysteriaObserved - hysteriaAllocate
	unknownAllocate := minUint64(unknownObserved, available)
	available -= unknownAllocate

	return realityCarry + realityAllocate,
		hysteriaCarry + hysteriaAllocate,
		available + unknownAllocate,
		saturatingSum(realityAhead, realityExcess),
		saturatingSum(hysteriaAhead, hysteriaExcess)
}

func snapshotOpenFlows(snapshot ClashSnapshot) map[string]OpenFlow {
	open := make(map[string]OpenFlow, len(snapshot.Connections))
	for _, connection := range snapshot.Connections {
		open[connection.ID] = OpenFlow{
			Protocol: classify(connection.Metadata.Type),
			Upload:   connection.Upload,
			Download: connection.Download,
			Start:    connection.Start.UTC(),
		}
	}
	return open
}

func activeFlowCounts(open map[string]OpenFlow) FlowCounts {
	var result FlowCounts
	for _, flow := range open {
		result.increment(flow.Protocol)
	}
	return result
}

func sameFlow(old OpenFlow, connection ClashConnection) bool {
	return old.Protocol == classify(connection.Metadata.Type) && old.Start.Equal(connection.Start.UTC())
}

func findConnection(connections []ClashConnection, id string) (ClashConnection, bool) {
	for _, connection := range connections {
		if connection.ID == id {
			return connection, true
		}
	}
	return ClashConnection{}, false
}

func sourceChanged(old, next SourceIdentity) bool {
	if old.zero() && next.zero() {
		return false
	}
	if old.Instance != next.Instance {
		return true
	}
	return old.Epoch != next.Epoch && (old.Epoch != "" || next.Epoch != "")
}

func countersDecreased(old ByteCounters, next ClashSnapshot) bool {
	return next.UploadTotal < old.Upload || next.DownloadTotal < old.Download
}

func sampleInterval(previous, next time.Time) time.Duration {
	if previous.IsZero() || !next.After(previous) {
		return 0
	}
	return next.Sub(previous)
}

func rates(delta ProtocolCounters, interval time.Duration) ProtocolRates {
	if interval <= 0 {
		return ProtocolRates{}
	}
	seconds := interval.Seconds()
	return ProtocolRates{
		Reality: RateCounters{
			Upload:   float64(delta.Reality.Upload) / seconds,
			Download: float64(delta.Reality.Download) / seconds,
		},
		Hysteria2: RateCounters{
			Upload:   float64(delta.Hysteria2.Upload) / seconds,
			Download: float64(delta.Hysteria2.Download) / seconds,
		},
		Unattributed: RateCounters{
			Upload:   float64(delta.Unattributed.Upload) / seconds,
			Download: float64(delta.Unattributed.Download) / seconds,
		},
	}
}

func mergeCounters(target *ProtocolCounters, delta ProtocolCounters) {
	for _, protocol := range protocolOrder {
		mergeBytes(target.counter(protocol), delta.For(protocol))
	}
}

func mergeBytes(target *ByteCounters, delta ByteCounters) {
	target.Upload = saturatingSum(target.Upload, delta.Upload)
	target.Download = saturatingSum(target.Download, delta.Download)
}

func minUint64(left, right uint64) uint64 {
	if left < right {
		return left
	}
	return right
}

func clonePersistentState(state PersistentState) PersistentState {
	clone := state
	clone.Open = make(map[string]OpenFlow, len(state.Open))
	for id, flow := range state.Open {
		clone.Open[id] = flow
	}
	clone.History = append([]MinutePoint(nil), state.History...)
	return clone
}
