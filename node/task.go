package node

import (
	"runtime"
	"time"

	"github.com/nsevo/v2sp/api/panel"
	"github.com/nsevo/v2sp/common/task"
	vCore "github.com/nsevo/v2sp/core"
	"github.com/nsevo/v2sp/limiter"
	log "github.com/sirupsen/logrus"
)

func (c *Controller) startTasks(node *panel.NodeInfo) {
	// fetch node info task
	c.nodeInfoMonitorPeriodic = &task.Task{
		Interval: node.PullInterval,
		Execute:  c.nodeInfoMonitor,
	}
	// fetch user list task
	c.userReportPeriodic = &task.Task{
		Interval: node.PushInterval,
		Execute:  c.reportUserTrafficTask,
	}
	log.WithField("tag", c.tag).Info("Start monitor node status")
	// delay to start nodeInfoMonitor
	_ = c.nodeInfoMonitorPeriodic.Start(false)
	log.WithField("tag", c.tag).Info("Start report node status")
	_ = c.userReportPeriodic.Start(false)
	if node.Security == panel.Tls {
		switch c.CertConfig.CertMode {
		case "none", "", "file", "self":
		default:
			c.renewCertPeriodic = &task.Task{
				Interval: time.Hour * 24,
				Execute:  c.renewCertTask,
			}
			log.WithField("tag", c.tag).Info("Start renew cert")
			// delay to start renewCert
			_ = c.renewCertPeriodic.Start(true)
		}
	}
	if c.LimitConfig.EnableDynamicSpeedLimit {
		c.traffic = make(map[string]int64)
		c.dynamicSpeedLimitPeriodic = &task.Task{
			Interval: time.Duration(c.LimitConfig.DynamicSpeedLimitConfig.Periodic) * time.Second,
			Execute:  c.SpeedChecker,
		}
		log.WithFields(log.Fields{
			"tag":     c.tag,
			"node_id": c.apiClient.NodeId,
			"period":  c.dynamicSpeedLimitPeriodic.Interval,
		}).Info("Start dynamic speed limit")
		_ = c.dynamicSpeedLimitPeriodic.Start(false)
	}
}

func (c *Controller) nodeInfoMonitor() (err error) {
	roundStart := time.Now()
	// get node info
	t0 := time.Now()
	newN, err := c.apiClient.GetNodeInfo()
	dNode := time.Since(t0)
	if err != nil {
		log.WithFields(log.Fields{
			"tag": c.tag,
			"err": err,
			"dur": dNode.Truncate(time.Millisecond),
		}).Warn("Get node info failed, will retry next interval")
		return nil
	}
	// get user info
	t1 := time.Now()
	newU, err := c.apiClient.GetUserList()
	dUsers := time.Since(t1)
	if err != nil {
		log.WithFields(log.Fields{
			"tag": c.tag,
			"err": err,
			"dur": dUsers.Truncate(time.Millisecond),
		}).Warn("Get user list failed, will retry next interval")
		return nil
	}
	// get user alive
	t2 := time.Now()
	newA, err := c.apiClient.GetUserAlive()
	dAlive := time.Since(t2)
	if err != nil {
		log.WithFields(log.Fields{
			"tag": c.tag,
			"err": err,
			"dur": dAlive.Truncate(time.Millisecond),
		}).Warn("Get alive list failed, will retry next interval")
		return nil
	}

	// Baseline per-round stats (low frequency)
	{
		var ms runtime.MemStats
		runtime.ReadMemStats(&ms)
		log.WithFields(log.Fields{
			"tag":        c.tag,
			"node_id":    c.apiClient.NodeId,
			"node_type":  c.apiClient.NodeType,
			"config_304": newN == nil,
			"users_304":  newU == nil,
			"alive_keys": len(newA),
			"dur_config": dNode.Truncate(time.Millisecond),
			"dur_users":  dUsers.Truncate(time.Millisecond),
			"dur_alive":  dAlive.Truncate(time.Millisecond),
			"dur_total":  time.Since(roundStart).Truncate(time.Millisecond),
			"goroutines": runtime.NumGoroutine(),
			"heap_alloc": ms.HeapAlloc,
			"sys":        ms.Sys,
		}).Debug("Node monitor round (baseline)")
	}

	if newN != nil {
		oldTag := c.tag
		newTag := oldTag
		c.info = newN
		// nodeInfo changed
		if newU != nil {
			c.userList = newU
		}
		if c.LimitConfig.EnableDynamicSpeedLimit {
		c.traffic = make(map[string]int64)
		} else {
			c.traffic = nil
		}
		// Remove old node
		log.WithField("tag", oldTag).Info("Node changed, reload")
		err = c.server.DelNode(oldTag)
		if err != nil {
			log.WithFields(log.Fields{
				"tag": oldTag,
				"err": err,
			}).Error("Delete node failed, will retry next interval")
			return nil
		}

		// Update limiter
		if len(c.Options.Name) == 0 {
			newTag = c.buildNodeTag(newN)
			// Remove old limiter (keyed by oldTag), then create a fresh limiter for newTag.
			// IMPORTANT: do not delete by newTag, or we may leak the old limiter.
			if newTag != oldTag {
				limiter.DeleteLimiter(oldTag)
			}
			// Switch tag only after old resources are removed.
			c.tag = newTag
			c.limiter = limiter.AddLimiter(newTag, &c.LimitConfig, c.userList, newA)
		}
		// update alive list
		if newA != nil {
			c.limiter.AliveList = newA
		}
		// Update rule
		err = c.limiter.UpdateRule(&newN.Rules)
		if err != nil {
			log.WithFields(log.Fields{
				"tag": c.tag,
				"err": err,
			}).Error("Update Rule failed")
			return nil
		}

		// check cert
		if newN.Security == panel.Tls {
			err = c.requestCert()
			if err != nil {
				log.WithFields(log.Fields{
					"tag": c.tag,
					"err": err,
				}).Error("Request cert failed")
				return nil
			}
		}
		// add new node
		err = c.server.AddNode(c.tag, newN, c.Options)
		if err != nil {
			log.WithFields(log.Fields{
				"tag": c.tag,
				"err": err,
			}).Error("Add node failed, will retry next interval")
			return nil
		}
		start := time.Now()
		addedTotal, err := c.server.AddUsers(&vCore.AddUsersParams{
			Tag:      c.tag,
			Users:    c.userList,
			NodeInfo: newN,
		})
		if err != nil {
			log.WithFields(log.Fields{
				"tag": c.tag,
				"err": err,
			}).Error("Add users failed")
			return nil
		}
		log.WithFields(log.Fields{
			"tag":      c.tag,
			"users":    addedTotal,
			"duration": time.Since(start).Truncate(time.Millisecond),
		}).Info("Users reloaded after node change")
		// Check interval
		if c.nodeInfoMonitorPeriodic.Interval != newN.PullInterval &&
			newN.PullInterval != 0 {
			c.nodeInfoMonitorPeriodic.Interval = newN.PullInterval
			c.nodeInfoMonitorPeriodic.Close()
			_ = c.nodeInfoMonitorPeriodic.Start(false)
		}
		if c.userReportPeriodic.Interval != newN.PushInterval &&
			newN.PushInterval != 0 {
			c.userReportPeriodic.Interval = newN.PushInterval
			c.userReportPeriodic.Close()
			_ = c.userReportPeriodic.Start(false)
		}
		// exit
		return nil
	}
	// update alive list
	if newA != nil {
		c.limiter.AliveList = newA
	}
	// node no changed, check users
	if len(newU) == 0 {
		return nil
	}
	deleted, added, updated := compareUserList(c.userList, newU)
	if len(deleted) > 0 {
		// have deleted users
		err = c.server.DelUsers(deleted, c.tag, c.info)
		if err != nil {
			log.WithFields(log.Fields{
				"tag": c.tag,
				"err": err,
			}).Error("Delete users failed")
			return nil
		}
	}
	if len(added) > 0 {
		// have added users
		_, err = c.server.AddUsers(&vCore.AddUsersParams{
			Tag:      c.tag,
			Users:    added,
			NodeInfo: c.info,
		})
		if err != nil {
			log.WithFields(log.Fields{
				"tag": c.tag,
				"err": err,
			}).Error("Add users failed")
			return nil
		}
	}
	if len(added) > 0 || len(deleted) > 0 {
		// update Limiter
		c.limiter.UpdateUser(c.tag, added, deleted)
		if err != nil {
			log.WithFields(log.Fields{
				"tag": c.tag,
				"err": err,
			}).Error("limiter users failed")
			return nil
		}
		// clear traffic record
		if c.LimitConfig.EnableDynamicSpeedLimit {
			for i := range deleted {
				delete(c.traffic, deleted[i].Uuid)
			}
		}
	}
	if len(updated) > 0 {
		// have users with updated limits
		c.limiter.UpdateUserLimits(c.tag, updated)
		log.WithField("tag", c.tag).Infof("%d users limits updated", len(updated))
	}
	c.userList = newU
	if len(added)+len(deleted)+len(updated) != 0 {
		log.WithField("tag", c.tag).
			Infof("%d user deleted, %d user added, %d user limits updated", len(deleted), len(added), len(updated))
	}
	return nil
}

func (c *Controller) SpeedChecker() error {
	for u, t := range c.traffic {
		if t >= c.LimitConfig.DynamicSpeedLimitConfig.Traffic {
			err := c.limiter.UpdateDynamicSpeedLimit(c.tag, u,
				c.LimitConfig.DynamicSpeedLimitConfig.SpeedLimit,
				time.Now().Add(time.Duration(c.LimitConfig.DynamicSpeedLimitConfig.ExpireTime)*time.Minute))
			log.WithField("err", err).Error("Update dynamic speed limit failed")
			delete(c.traffic, u)
		}
	}
	return nil
}
