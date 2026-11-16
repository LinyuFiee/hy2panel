package traffic

import (
	"fmt"
	"time"
)

const (
	// StateSchemaVersion is the on-disk traffic state schema understood by this package.
	StateSchemaVersion = 1
	// MaxHistoryPoints is seven days of one-minute buckets.
	MaxHistoryPoints = 7 * 24 * 60
	// MaxOpenFlows bounds both persisted checkpoints and accepted snapshots.
	MaxOpenFlows = 32768
)

// Protocol is a deliberately small traffic attribution category. It never
// contains a peer address, destination, routing rule, or arbitrary API label.
type Protocol string

const (
	ProtocolReality      Protocol = "reality"
	ProtocolHysteria2    Protocol = "hysteria2"
	ProtocolUnattributed Protocol = "unattributed"
)

var protocolOrder = [...]Protocol{ProtocolReality, ProtocolHysteria2, ProtocolUnattributed}

func (p Protocol) valid() bool {
	switch p {
	case ProtocolReality, ProtocolHysteria2, ProtocolUnattributed:
		return true
	default:
		return false
	}
}

// ByteCounters contains traffic in the same direction names used by Clash's
// /connections API.
type ByteCounters struct {
	Upload   uint64 `json:"upload"`
	Download uint64 `json:"download"`
}

// ProtocolCounters contains all durable traffic categories. Total returns the
// authoritative aggregate represented by these categories.
type ProtocolCounters struct {
	Reality      ByteCounters `json:"reality"`
	Hysteria2    ByteCounters `json:"hysteria2"`
	Unattributed ByteCounters `json:"unattributed"`
}

func (c ProtocolCounters) For(protocol Protocol) ByteCounters {
	switch protocol {
	case ProtocolReality:
		return c.Reality
	case ProtocolHysteria2:
		return c.Hysteria2
	default:
		return c.Unattributed
	}
}

func (c *ProtocolCounters) counter(protocol Protocol) *ByteCounters {
	switch protocol {
	case ProtocolReality:
		return &c.Reality
	case ProtocolHysteria2:
		return &c.Hysteria2
	default:
		return &c.Unattributed
	}
}

func (c ProtocolCounters) Total() ByteCounters {
	return ByteCounters{
		Upload:   saturatingSum(c.Reality.Upload, c.Hysteria2.Upload, c.Unattributed.Upload),
		Download: saturatingSum(c.Reality.Download, c.Hysteria2.Download, c.Unattributed.Download),
	}
}

// FlowCounts reports currently visible proxy flows by attribution category.
type FlowCounts struct {
	Reality      uint64 `json:"reality"`
	Hysteria2    uint64 `json:"hysteria2"`
	Unattributed uint64 `json:"unattributed"`
}

func (c FlowCounts) Total() uint64 {
	return saturatingSum(c.Reality, c.Hysteria2, c.Unattributed)
}

func (c *FlowCounts) increment(protocol Protocol) {
	switch protocol {
	case ProtocolReality:
		c.Reality++
	case ProtocolHysteria2:
		c.Hysteria2++
	default:
		c.Unattributed++
	}
}

// ProtocolRates contains bytes per second averaged over the latest successful
// sampling interval.
type ProtocolRates struct {
	Reality      RateCounters `json:"reality"`
	Hysteria2    RateCounters `json:"hysteria2"`
	Unattributed RateCounters `json:"unattributed"`
}

// RateCounters is a floating-point live rate, not a durable byte counter.
type RateCounters struct {
	Upload   float64 `json:"upload"`
	Download float64 `json:"download"`
}

// SourceIdentity lets an integration attach a stable source instance and a
// process/invocation epoch to a sample. ClashClient leaves it empty unless the
// caller sets it. A change causes the collector to establish a fresh baseline.
type SourceIdentity struct {
	Instance string `json:"instance,omitempty"`
	Epoch    string `json:"epoch,omitempty"`
}

func (s SourceIdentity) zero() bool {
	return s.Instance == "" && s.Epoch == ""
}

// ClashMetadata is the only metadata retained from a Clash connection. In
// particular, it intentionally has no address, host, process, chain, or rule.
type ClashMetadata struct {
	Type string `json:"type"`
}

// ClashConnection is the minimal per-flow /connections input model.
type ClashConnection struct {
	ID       string        `json:"id"`
	Metadata ClashMetadata `json:"metadata"`
	Upload   uint64        `json:"upload"`
	Download uint64        `json:"download"`
	Start    time.Time     `json:"start"`
}

// ClashSnapshot is the minimal top-level /connections input model. Source is
// local integration metadata and is never decoded from the HTTP response.
type ClashSnapshot struct {
	DownloadTotal uint64            `json:"downloadTotal"`
	UploadTotal   uint64            `json:"uploadTotal"`
	Connections   []ClashConnection `json:"connections"`
	Source        SourceIdentity    `json:"-"`
}

// LiveSnapshot is safe to read concurrently through Collector.Snapshot.
type LiveSnapshot struct {
	SampledAt   time.Time        `json:"sampled_at"`
	LastSuccess time.Time        `json:"last_success,omitempty"`
	Available   bool             `json:"available"`
	Stale       bool             `json:"stale"`
	Traffic     ProtocolCounters `json:"traffic"`
	Total       ByteCounters     `json:"total"`
	Rates       ProtocolRates    `json:"rates"`
	ActiveFlows FlowCounts       `json:"active_flows"`
}

// MinutePoint stores logical byte increments assigned to one UTC minute. Gap
// remains true once any expected sample in that minute was unavailable.
type MinutePoint struct {
	Minute  time.Time        `json:"minute"`
	Traffic ProtocolCounters `json:"traffic"`
	Total   ByteCounters     `json:"total"`
	Gap     bool             `json:"gap"`
}

// HistoryRange is one of the bounded ranges accepted by Collector.History.
type HistoryRange string

const (
	History1Hour   HistoryRange = "1h"
	History6Hours  HistoryRange = "6h"
	History24Hours HistoryRange = "24h"
	History7Days   HistoryRange = "7d"
)

func (r HistoryRange) duration() (time.Duration, error) {
	switch r {
	case History1Hour:
		return time.Hour, nil
	case History6Hours:
		return 6 * time.Hour, nil
	case History24Hours:
		return 24 * time.Hour, nil
	case History7Days:
		return 7 * 24 * time.Hour, nil
	default:
		return 0, fmt.Errorf("unsupported traffic history range %q", r)
	}
}

// OpenFlow is the privacy-minimal durable baseline for one active connection.
type OpenFlow struct {
	Protocol Protocol  `json:"protocol"`
	Upload   uint64    `json:"upload"`
	Download uint64    `json:"download"`
	Start    time.Time `json:"start"`
}

// PersistentState is the complete, bounded checkpoint format. It contains no
// secret and no remote/local address, domain, process, chain, or routing rule.
type PersistentState struct {
	SchemaVersion int                 `json:"schema_version"`
	SavedAt       time.Time           `json:"saved_at"`
	Initialized   bool                `json:"initialized"`
	Source        SourceIdentity      `json:"source"`
	Global        ByteCounters        `json:"global"`
	Traffic       ProtocolCounters    `json:"traffic"`
	Ahead         ProtocolCounters    `json:"ahead"`
	Open          map[string]OpenFlow `json:"open"`
	History       []MinutePoint       `json:"history"`
}

func newPersistentState() PersistentState {
	return PersistentState{
		SchemaVersion: StateSchemaVersion,
		Open:          make(map[string]OpenFlow),
	}
}

func classify(label string) Protocol {
	switch label {
	case "vless/reality":
		return ProtocolReality
	case "hysteria2/hysteria2":
		return ProtocolHysteria2
	default:
		return ProtocolUnattributed
	}
}

func validSourceIdentity(source SourceIdentity) bool {
	return len(source.Instance) <= 256 && len(source.Epoch) <= 256
}

func saturatingSum(values ...uint64) uint64 {
	var total uint64
	for _, value := range values {
		next := total + value
		if next < total {
			return ^uint64(0)
		}
		total = next
	}
	return total
}
