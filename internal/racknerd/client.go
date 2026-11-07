package racknerd

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const defaultEndpoint = "https://ctrl.racknerd.com/api/client/command.php"

type Info struct {
	BandwidthTotal uint64    `json:"bandwidth_total"`
	BandwidthUsed  uint64    `json:"bandwidth_used"`
	BandwidthFree  uint64    `json:"bandwidth_free"`
	PercentUsed    float64   `json:"percent_used"`
	Hostname       string    `json:"hostname"`
	VMState        string    `json:"vm_state"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type Client struct {
	Endpoint   string
	HTTPClient *http.Client
}

func (c *Client) Info(ctx context.Context, key, hash string) (Info, error) {
	var result Info
	if key == "" || hash == "" {
		return result, errors.New("RackNerd API 凭据未配置")
	}
	endpoint := c.Endpoint
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	form := url.Values{
		"key":    {key},
		"hash":   {hash},
		"action": {"info"},
		"rdtype": {"json"},
		"bw":     {"1"},
		"status": {"1"},
	}
	request, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return result, err
	}
	request.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpClient := c.HTTPClient
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 15 * time.Second}
	}
	response, err := httpClient.Do(request)
	if err != nil {
		return result, err
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		return result, fmt.Errorf("RackNerd API 返回 HTTP %d", response.StatusCode)
	}
	var payload struct {
		Status    string `json:"status"`
		StatusMsg string `json:"statusmsg"`
		BW        string `json:"bw"`
		Hostname  string `json:"hostname"`
		VMState   string `json:"vmstat"`
	}
	decoder := json.NewDecoder(io.LimitReader(response.Body, 1<<20))
	if err := decoder.Decode(&payload); err != nil {
		return result, errors.New("RackNerd API 响应不是有效 JSON")
	}
	if payload.Status != "success" {
		message := strings.TrimSpace(payload.StatusMsg)
		if message == "" {
			message = "未知错误"
		}
		if len(message) > 512 {
			message = message[:512]
		}
		return result, fmt.Errorf("RackNerd API: %s", message)
	}
	total, used, free, percent, err := parseUsage(payload.BW)
	if err != nil {
		return result, err
	}
	result = Info{
		BandwidthTotal: total,
		BandwidthUsed:  used,
		BandwidthFree:  free,
		PercentUsed:    percent,
		Hostname:       payload.Hostname,
		VMState:        payload.VMState,
		UpdatedAt:      time.Now().UTC(),
	}
	return result, nil
}

func parseUsage(value string) (uint64, uint64, uint64, float64, error) {
	parts := strings.Split(value, ",")
	if len(parts) != 4 {
		return 0, 0, 0, 0, errors.New("RackNerd 流量字段格式无效")
	}
	values := make([]uint64, 3)
	for i := 0; i < 3; i++ {
		parsed, err := strconv.ParseUint(strings.TrimSpace(parts[i]), 10, 64)
		if err != nil {
			return 0, 0, 0, 0, errors.New("RackNerd 流量字段包含无效字节数")
		}
		values[i] = parsed
	}
	percent, err := strconv.ParseFloat(strings.TrimSpace(strings.TrimSuffix(parts[3], "%")), 64)
	if err != nil || percent < 0 || percent > 100 {
		return 0, 0, 0, 0, errors.New("RackNerd 流量百分比无效")
	}
	if values[0] < values[1] || values[0] < values[2] {
		return 0, 0, 0, 0, errors.New("RackNerd 流量总量小于已用量或剩余量")
	}
	return values[0], values[1], values[2], percent, nil
}
