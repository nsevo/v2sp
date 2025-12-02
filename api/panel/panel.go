package panel

import (
	"errors"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/sirupsen/logrus"

	"github.com/go-resty/resty/v2"
	"github.com/nsevo/v2sp/conf"
)

// Panel is the interface for different panel's api.

type Client struct {
	client           *resty.Client
	APIHost          string
	APISendIP        string
	Token            string
	NodeType         string
	NodeId           int
	nodeEtag         string
	userEtag         string
	responseBodyHash string
	UserList         *UserListBody
	AliveMap         *AliveMap
}

func New(c *conf.ApiConfig) (*Client, error) {
	var client *resty.Client
	if c.APISendIP != "" {
		client = resty.NewWithLocalAddr(&net.TCPAddr{
			IP: net.ParseIP(c.APISendIP),
		})
	} else {
		client = resty.New()
	}
	client.SetRetryCount(3)
	if c.Timeout > 0 {
		client.SetTimeout(time.Duration(c.Timeout) * time.Second)
	} else {
		client.SetTimeout(5 * time.Second)
	}
	client.OnError(func(req *resty.Request, err error) {
		var v *resty.ResponseError
		if errors.As(err, &v) {
			logrus.Error(v.Err)
		}
	})
	client.SetBaseURL(c.APIHost)

	// Normalize node type (can be empty, will be auto-detected from API)
	nodeType := strings.ToLower(c.NodeType)
	switch nodeType {
	case "v2ray":
		nodeType = "vmess"
	case "", "vmess", "trojan", "shadowsocks", "vless", "hysteria", "hysteria2":
		// empty is allowed - will be auto-detected from API
	default:
		return nil, fmt.Errorf("unsupported Node type: %s", c.NodeType)
	}

	// Set query params - node_type is optional
	params := map[string]string{
		"node_id": strconv.Itoa(c.NodeID),
		"token":   c.Key,
	}
	if nodeType != "" {
		params["node_type"] = nodeType
	}
	client.SetQueryParams(params)

	return &Client{
		client:    client,
		Token:     c.Key,
		APIHost:   c.APIHost,
		APISendIP: c.APISendIP,
		NodeType:  nodeType, // may be empty, will be set after first API call
		NodeId:    c.NodeID,
		UserList:  &UserListBody{},
		AliveMap:  &AliveMap{},
	}, nil
}
