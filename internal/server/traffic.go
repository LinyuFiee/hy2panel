package server

import (
	"net/http"
	"time"

	"hy2panel/internal/traffic"
)

type trafficLiveResponse struct {
	Available           bool                     `json:"available"`
	Stale               bool                     `json:"stale"`
	SampledAt           time.Time                `json:"sampled_at,omitempty"`
	LastSuccess         time.Time                `json:"last_success,omitempty"`
	CollectionError     string                   `json:"error,omitempty"`
	Traffic             traffic.ProtocolCounters `json:"traffic"`
	Total               traffic.ByteCounters     `json:"total"`
	Rates               traffic.ProtocolRates    `json:"rates"`
	ActiveFlows         traffic.FlowCounts       `json:"active_flows"`
	AttributionCoverage float64                  `json:"attribution_coverage"`
}

func (s *Server) systemStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]any{
		"status":    readStatus(r.Context()),
		"speedtest": s.speedtest.State(),
	})
}

func (s *Server) trafficLive(w http.ResponseWriter, _ *http.Request) {
	if s.traffic == nil {
		writeJSON(w, http.StatusOK, trafficLiveResponse{Stale: true, CollectionError: s.trafficErr})
		return
	}
	snapshot := s.traffic.Snapshot()
	attributed := snapshot.Traffic.Reality.Upload + snapshot.Traffic.Reality.Download + snapshot.Traffic.Hysteria2.Upload + snapshot.Traffic.Hysteria2.Download
	total := snapshot.Total.Upload + snapshot.Total.Download
	coverage := float64(0)
	if total > 0 {
		coverage = float64(attributed) / float64(total)
		if coverage > 1 {
			coverage = 1
		}
	}
	writeJSON(w, http.StatusOK, trafficLiveResponse{
		Available:           snapshot.Available,
		Stale:               snapshot.Stale,
		SampledAt:           snapshot.SampledAt,
		LastSuccess:         snapshot.LastSuccess,
		Traffic:             snapshot.Traffic,
		Total:               snapshot.Total,
		Rates:               snapshot.Rates,
		ActiveFlows:         snapshot.ActiveFlows,
		AttributionCoverage: coverage,
	})
}

func (s *Server) trafficHistory(w http.ResponseWriter, r *http.Request) {
	if s.traffic == nil {
		writeJSON(w, http.StatusOK, map[string]any{"range": "", "points": []traffic.MinutePoint{}, "available": false, "error": s.trafficErr})
		return
	}
	value := r.URL.Query().Get("range")
	if value == "" {
		value = string(traffic.History1Hour)
	}
	historyRange := traffic.HistoryRange(value)
	points, err := s.traffic.History(historyRange)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"range": value, "points": points, "available": true})
}
