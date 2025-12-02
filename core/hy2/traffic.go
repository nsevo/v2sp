package hy2

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/nsevo/v2sp/api/panel"
)

// TrafficStatsClient is a client for Hysteria2's Traffic Stats API
type TrafficStatsClient struct {
	baseURL    string
	secret     string
	httpClient *http.Client
}

// TrafficStatsResponse represents the response from traffic stats API
type TrafficStatsResponse struct {
	Users map[string]UserTrafficStats `json:"users"`
}

// UserTrafficStats represents traffic statistics for a user
type UserTrafficStats struct {
	Tx uint64 `json:"tx"` // Upload (bytes sent to remote)
	Rx uint64 `json:"rx"` // Download (bytes received from remote)
}

// OnlineUsersResponse represents the response from online users API
type OnlineUsersResponse struct {
	Users []string `json:"users"`
}

// NewTrafficStatsClient creates a new traffic stats client
func NewTrafficStatsClient(baseURL string, secret string) *TrafficStatsClient {
	return &TrafficStatsClient{
		baseURL: baseURL,
		secret:  secret,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetTraffic retrieves traffic statistics for all users
func (c *TrafficStatsClient) GetTraffic(clear bool) (*TrafficStatsResponse, error) {
	url := fmt.Sprintf("%s/traffic", c.baseURL)
	if clear {
		url += "?clear=1"
	}

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.secret != "" {
		req.Header.Set("Authorization", c.secret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get traffic: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("traffic API returned %d: %s", resp.StatusCode, string(body))
	}

	var result TrafficStatsResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return &result, nil
}

// GetOnlineUsers retrieves the list of online users
func (c *TrafficStatsClient) GetOnlineUsers() (*OnlineUsersResponse, error) {
	url := fmt.Sprintf("%s/online", c.baseURL)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}

	if c.secret != "" {
		req.Header.Set("Authorization", c.secret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get online users: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("online API returned %d: %s", resp.StatusCode, string(body))
	}

	var result OnlineUsersResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %v", err)
	}

	return &result, nil
}

// KickUser kicks a user (forces disconnect)
func (c *TrafficStatsClient) KickUser(uuid string) error {
	url := fmt.Sprintf("%s/kick", c.baseURL)

	req, err := http.NewRequest("POST", url, nil)
	if err != nil {
		return err
	}

	q := req.URL.Query()
	q.Add("id", uuid)
	req.URL.RawQuery = q.Encode()

	if c.secret != "" {
		req.Header.Set("Authorization", c.secret)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to kick user: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("kick API returned %d: %s", resp.StatusCode, string(body))
	}

	return nil
}

// ConvertToUserTraffic converts Hysteria2 traffic stats to v2sp format
func ConvertToUserTraffic(stats *TrafficStatsResponse, userMap map[string]int) []panel.UserTraffic {
	var result []panel.UserTraffic

	for uuid, traffic := range stats.Users {
		if uid, ok := userMap[uuid]; ok {
			// Only include if there's actual traffic
			if traffic.Tx > 0 || traffic.Rx > 0 {
				result = append(result, panel.UserTraffic{
					UID:      uid,
					Upload:   int64(traffic.Tx),
					Download: int64(traffic.Rx),
				})
			}
		}
	}

	return result
}
