package conf

import (
	"encoding/json"
	"fmt"
	"strings"
)

type CoreConfig struct {
	Type       string      `json:"Type"`
	Name       string      `json:"Name"`
	XrayConfig *XrayConfig `json:"-"`
	Hy2Config  *Hy2Config  `json:"-"`
}

type _CoreConfig CoreConfig

func (c *CoreConfig) UnmarshalJSON(b []byte) error {
	err := json.Unmarshal(b, (*_CoreConfig)(c))
	if err != nil {
		return err
	}

	// Normalize type to lowercase
	c.Type = strings.ToLower(c.Type)

	// Default to xray if not specified
	if c.Type == "" {
		c.Type = "xray"
	}

	switch c.Type {
	case "xray":
		c.XrayConfig = NewXrayConfig()
		return json.Unmarshal(b, c.XrayConfig)
	case "hysteria2", "hy2":
		c.Type = "hysteria2"
		c.Hy2Config = NewHy2Config()
		return json.Unmarshal(b, c.Hy2Config)
	default:
		return fmt.Errorf("unsupported core type: %s (supported: xray, hysteria2)", c.Type)
	}
}
