package node

import (
	"fmt"

	"github.com/nsevo/v2sp/api/panel"
	"github.com/nsevo/v2sp/conf"
	vCore "github.com/nsevo/v2sp/core"
	log "github.com/sirupsen/logrus"
)

type Node struct {
	controllers []*Controller
}

func New() *Node {
	return &Node{}
}

func (n *Node) Start(nodes []conf.NodeConfig, core vCore.Core) error {
	n.controllers = make([]*Controller, 0, len(nodes))

	var successCount, failCount int

	for i := range nodes {
		p, err := panel.New(&nodes[i].ApiConfig)
		if err != nil {
			log.WithFields(log.Fields{
				"node_id": nodes[i].ApiConfig.NodeID,
				"error":   err,
			}).Error("Failed to create panel client, skipping node")
			failCount++
			continue
		}

		// Register controller service
		controller := NewController(core, p, &nodes[i].Options)
		err = controller.Start()
		if err != nil {
			log.WithFields(log.Fields{
				"api_host": nodes[i].ApiConfig.APIHost,
				"node_id":  nodes[i].ApiConfig.NodeID,
				"error":    err,
			}).Error("Failed to start node controller, skipping node")
			failCount++
			continue
		}

		n.controllers = append(n.controllers, controller)
		successCount++

		log.WithFields(log.Fields{
			"node_id": nodes[i].ApiConfig.NodeID,
		}).Info("Node started successfully")
	}

	// Log summary
	log.WithFields(log.Fields{
		"success": successCount,
		"failed":  failCount,
		"total":   len(nodes),
	}).Info("Node startup complete")

	// Only return error if ALL nodes failed
	if successCount == 0 && len(nodes) > 0 {
		return fmt.Errorf("all %d nodes failed to start", failCount)
	}

	return nil
}

func (n *Node) Close() {
	for _, c := range n.controllers {
		if c == nil {
			continue
		}
		err := c.Close()
		if err != nil {
			log.WithError(err).Error("Failed to close node controller")
		}
	}
	n.controllers = nil
}
