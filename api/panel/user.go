package panel

import (
	"encoding/json"
	"fmt"
)

type OnlineUser struct {
	UID int
	IP  string
}

type UserInfo struct {
	Id          int    `json:"id"`
	Uuid        string `json:"uuid"`
	SpeedLimit  int    `json:"speed_limit"`
	DeviceLimit int    `json:"device_limit"`
}

type UserListBody struct {
	Users []UserInfo `json:"users"`
}

type AliveMap struct {
	Alive map[int]int `json:"alive"`
}

const (
	actionUserList    = "user"
	actionAliveList   = "alivelist"
	actionPushTraffic = "push"
	actionAliveReport = "alive"
)

// GetUserList will pull user from panel
func (c *Client) GetUserList() ([]UserInfo, error) {
	r, err := c.client.R().
		SetHeader("If-None-Match", c.userEtag).
		ForceContentType("application/json").
		Get(c.assembleURL(actionUserList))

	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}

	if r.StatusCode() == 304 {
		return nil, nil
	}

	if err = c.checkResponse(r, actionUserList, err); err != nil {
		return nil, err
	}

	userlist := &UserListBody{}
	if err := json.Unmarshal(r.Body(), userlist); err != nil {
		return nil, fmt.Errorf("decode user list error: %w", err)
	}

	c.userEtag = r.Header().Get("ETag")
	return userlist.Users, nil
}

// GetUserAlive will fetch the alive_ip count for users
func (c *Client) GetUserAlive() (map[int]int, error) {
	c.AliveMap = &AliveMap{}
	r, err := c.client.R().
		ForceContentType("application/json").
		Get(c.assembleURL(actionAliveList))

	if err != nil || r.StatusCode() >= 399 {
		c.AliveMap.Alive = make(map[int]int)
		return c.AliveMap.Alive, nil
	}

	if err := json.Unmarshal(r.Body(), c.AliveMap); err != nil {
		fmt.Printf("unmarshal user alive list error: %s", err)
		c.AliveMap.Alive = make(map[int]int)
	}

	return c.AliveMap.Alive, nil
}

type UserTraffic struct {
	UID      int
	Upload   int64
	Download int64
}

// ReportUserTraffic reports the user traffic
func (c *Client) ReportUserTraffic(userTraffic []UserTraffic) error {
	data := make(map[int][]int64, len(userTraffic))
	for i := range userTraffic {
		data[userTraffic[i].UID] = []int64{userTraffic[i].Upload, userTraffic[i].Download}
	}
	r, err := c.client.R().
		SetBody(data).
		ForceContentType("application/json").
		Post(c.assembleURL(actionPushTraffic))
	err = c.checkResponse(r, actionPushTraffic, err)
	if err != nil {
		return err
	}
	return nil
}

func (c *Client) ReportNodeOnlineUsers(data *map[int][]string) error {
	r, err := c.client.R().
		SetBody(data).
		ForceContentType("application/json").
		Post(c.assembleURL(actionAliveReport))
	if err = c.checkResponse(r, actionAliveReport, err); err != nil {
		return err
	}
	return nil
}
