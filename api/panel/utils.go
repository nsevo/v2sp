package panel

import (
	"fmt"
	"strings"

	"github.com/go-resty/resty/v2"
)

// Debug set the client debug for client
func (c *Client) Debug() {
	c.client.SetDebug(true)
}

func (c *Client) assembleURL(action string) string {
	if action == "" {
		return c.APIHost
	}
	separator := "?"
	switch {
	case strings.Contains(c.APIHost, "?") && !strings.HasSuffix(c.APIHost, "?") && !strings.HasSuffix(c.APIHost, "&"):
		separator = "&"
	case strings.HasSuffix(c.APIHost, "?") || strings.HasSuffix(c.APIHost, "&"):
		separator = ""
	}
	return fmt.Sprintf("%s%vaction=%s", c.APIHost, separator, action)
}
func (c *Client) checkResponse(res *resty.Response, action string, err error) error {
	if err != nil {
		return fmt.Errorf("request %s failed: %s", c.assembleURL(action), err)
	}
	if res.StatusCode() >= 400 {
		body := res.Body()
		return fmt.Errorf("request %s failed: %s", c.assembleURL(action), string(body))
	}
	return nil
}
