package node

import (
	"time"

	"github.com/nsevo/v2sp/api/panel"
	log "github.com/sirupsen/logrus"
)

func (c *Controller) reportUserTrafficTask() (err error) {
	roundStart := time.Now()
	userTraffic, _ := c.server.GetUserTrafficSlice(c.tag, true)
	if len(userTraffic) > 0 {
		t0 := time.Now()
		err = c.apiClient.ReportUserTraffic(userTraffic)
		d := time.Since(t0)
		if err != nil {
			log.WithFields(log.Fields{
				"tag": c.tag,
				"err": err,
				"dur": d.Truncate(time.Millisecond),
			}).Warn("Report user traffic failed")
		} else {
			log.WithField("tag", c.tag).Infof("Report %d users traffic", len(userTraffic))
			// Debug log disabled for large user counts to avoid massive log output
			// log.WithField("tag", c.tag).Debugf("User traffic: %+v", userTraffic)
		}
	}

	if onlineDevice, err := c.limiter.GetOnlineDevice(); err != nil {
		log.WithField("err", err).Warn("Get online device failed")
	} else if len(*onlineDevice) > 0 {
		// Only report user has traffic > threshold to allow ping test
		// Pre-allocate maps with estimated capacity
		nocountUID := make(map[int]struct{}, len(userTraffic))
		for _, traffic := range userTraffic {
			total := traffic.Upload + traffic.Download
			if total < int64(c.Options.DeviceOnlineMinTraffic*1000) {
				nocountUID[traffic.UID] = struct{}{}
			}
		}

		// Pre-allocate result slice with capacity
		result := make([]panel.OnlineUser, 0, len(*onlineDevice))
		for _, online := range *onlineDevice {
			if _, ok := nocountUID[online.UID]; !ok {
				result = append(result, online)
			}
		}

		// Build data map
		data := make(map[int][]string, len(result))
		for _, onlineuser := range result {
			// json structure: { UID1:["ip1","ip2"],UID2:["ip3","ip4"] }
			data[onlineuser.UID] = append(data[onlineuser.UID], onlineuser.IP)
		}

		if err = c.apiClient.ReportNodeOnlineUsers(&data); err != nil {
			log.WithFields(log.Fields{
				"tag": c.tag,
				"err": err,
			}).Warn("Report online users failed")
		} else {
			log.WithField("tag", c.tag).Infof("Total %d online users, %d Reported", len(*onlineDevice), len(result))
			// Debug log disabled for large user counts to avoid massive log output
			// log.WithField("tag", c.tag).Debugf("Online users: %+v", data)
		}
	}

	userTraffic = nil
	log.WithFields(log.Fields{
		"tag": c.tag,
		"dur": time.Since(roundStart).Truncate(time.Millisecond),
	}).Debug("Report round finished (baseline)")
	return nil
}

func compareUserList(old, new []panel.UserInfo) (deleted, added, updated []panel.UserInfo) {
	// Use UUID as the unique key to track users
	oldMap := make(map[string]panel.UserInfo, len(old))
	for _, user := range old {
		oldMap[user.Uuid] = user
	}

	// Pre-allocate slices with estimated capacity to reduce allocations
	// Most updates have small deltas, so we estimate 10% change rate
	estimatedChanges := len(new) / 10
	if estimatedChanges < 100 {
		estimatedChanges = 100
	}
	added = make([]panel.UserInfo, 0, estimatedChanges)
	updated = make([]panel.UserInfo, 0, estimatedChanges)
	deleted = make([]panel.UserInfo, 0, estimatedChanges)

	for _, newUser := range new {
		if oldUser, exists := oldMap[newUser.Uuid]; exists {
			// User exists, check if any limits have changed
			limitsChanged := oldUser.SpeedLimit != newUser.SpeedLimit ||
				oldUser.DeviceLimit != newUser.DeviceLimit

			if limitsChanged {
				// User limits changed, mark for update
				updated = append(updated, newUser)
			}
			// Remove from map so we know this user wasn't deleted
			delete(oldMap, newUser.Uuid)
		} else {
			// New user, mark for addition
			added = append(added, newUser)
		}
	}

	// Any remaining users in oldMap were deleted
	for _, user := range oldMap {
		deleted = append(deleted, user)
	}

	return deleted, added, updated
}
