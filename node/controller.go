package node

import (
	"fmt"
	"time"

	"github.com/nsevo/v2sp/api/panel"
	"github.com/nsevo/v2sp/common/task"
	"github.com/nsevo/v2sp/conf"
	vCore "github.com/nsevo/v2sp/core"
	"github.com/nsevo/v2sp/limiter"
	log "github.com/sirupsen/logrus"
)

type Controller struct {
	server                    vCore.Core
	apiClient                 *panel.Client
	tag                       string
	limiter                   *limiter.Limiter
	traffic                   map[string]int64
	userList                  []panel.UserInfo
	aliveMap                  map[int]int
	info                      *panel.NodeInfo
	nodeInfoMonitorPeriodic   *task.Task
	userReportPeriodic        *task.Task
	renewCertPeriodic         *task.Task
	dynamicSpeedLimitPeriodic *task.Task
	onlineIpReportPeriodic    *task.Task
	*conf.Options
}

const userImportBatchSize = 2000

// NewController return a Node controller with default parameters.
func NewController(server vCore.Core, api *panel.Client, config *conf.Options) *Controller {
	controller := &Controller{
		server:    server,
		Options:   config,
		apiClient: api,
	}
	return controller
}

// Start implement the Start() function of the service interface
func (c *Controller) Start() error {
	// First fetch Node Info
	var err error
	node, err := c.apiClient.GetNodeInfo()
	if err != nil {
		return fmt.Errorf("get node info error: %s", err)
	}
	// Update user
	c.userList, err = c.apiClient.GetUserList()
	if err != nil {
		return fmt.Errorf("get user list error: %s", err)
	}
	// Allow empty user list - users will be added when available
	c.aliveMap, err = c.apiClient.GetUserAlive()
	if err != nil {
		return fmt.Errorf("failed to get user alive list: %s", err)
	}
	if len(c.Options.Name) == 0 {
		c.tag = c.buildNodeTag(node)
	} else {
		c.tag = c.Options.Name
	}

	// add limiter
	l := limiter.AddLimiter(c.tag, &c.LimitConfig, c.userList, c.aliveMap)
	// add rule limiter
	if err = l.UpdateRule(&node.Rules); err != nil {
		return fmt.Errorf("update rule error: %s", err)
	}
	c.limiter = l
	if node.Security == panel.Tls {
		err = c.requestCert()
		if err != nil {
			return fmt.Errorf("request cert error: %s", err)
		}
	}
	// Add new tag
	err = c.server.AddNode(c.tag, node, c.Options)
	if err != nil {
		return fmt.Errorf("add new node error: %s", err)
	}
	if len(c.userList) > userImportBatchSize {
		log.WithFields(log.Fields{
			"tag":        c.tag,
			"user_count": len(c.userList),
			"batch_size": userImportBatchSize,
		}).Info("Large user list detected, importing in batches - please wait")
	} else {
		log.WithFields(log.Fields{
			"tag":        c.tag,
			"user_count": len(c.userList),
		}).Info("Importing users from panel")
	}
	added, duration, err := c.addUsersInBatches(node, c.userList)
	if err != nil {
		return fmt.Errorf("add users error: %s", err)
	}
	log.WithFields(log.Fields{
		"tag":      c.tag,
		"duration": duration.Truncate(time.Millisecond),
	}).Infof("Added %d new users", added)
	c.info = node
	c.startTasks(node)
	return nil
}

// Close implement the Close() function of the service interface
func (c *Controller) Close() error {
	limiter.DeleteLimiter(c.tag)
	if c.nodeInfoMonitorPeriodic != nil {
		c.nodeInfoMonitorPeriodic.Close()
	}
	if c.userReportPeriodic != nil {
		c.userReportPeriodic.Close()
	}
	if c.renewCertPeriodic != nil {
		c.renewCertPeriodic.Close()
	}
	if c.dynamicSpeedLimitPeriodic != nil {
		c.dynamicSpeedLimitPeriodic.Close()
	}
	if c.onlineIpReportPeriodic != nil {
		c.onlineIpReportPeriodic.Close()
	}
	err := c.server.DelNode(c.tag)
	if err != nil {
		return fmt.Errorf("del node error: %s", err)
	}
	return nil
}

func (c *Controller) buildNodeTag(node *panel.NodeInfo) string {
	return fmt.Sprintf("[%s]-%s:%d", c.apiClient.APIHost, node.Type, node.Id)
}

func (c *Controller) addUsersInBatches(node *panel.NodeInfo, users []panel.UserInfo) (int, time.Duration, error) {
	total := len(users)
	if total == 0 {
		return 0, 0, nil
	}
	batchSize := userImportBatchSize
	if batchSize <= 0 || batchSize > total {
		batchSize = total
	}
	start := time.Now()
	totalAdded := 0
	for startIdx := 0; startIdx < total; startIdx += batchSize {
		endIdx := startIdx + batchSize
		if endIdx > total {
			endIdx = total
		}
		added, err := c.server.AddUsers(&vCore.AddUsersParams{
			Tag:      c.tag,
			Users:    users[startIdx:endIdx],
			NodeInfo: node,
		})
		if err != nil {
			return totalAdded, 0, err
		}
		totalAdded += added
		if total > batchSize {
			log.WithFields(log.Fields{
				"tag":      c.tag,
				"batch":    fmt.Sprintf("%d-%d", startIdx+1, endIdx),
				"progress": fmt.Sprintf("%d/%d", totalAdded, total),
			}).Info("User import progress")
		}
	}
	return totalAdded, time.Since(start), nil
}
