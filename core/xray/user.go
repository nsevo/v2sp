package xray

import (
	"context"
	"fmt"
	"time"

	"github.com/nsevo/v2sp/api/panel"
	"github.com/nsevo/v2sp/common/counter"
	"github.com/nsevo/v2sp/common/format"
	vCore "github.com/nsevo/v2sp/core"
	"github.com/nsevo/v2sp/core/xray/app/mydispatcher"
	log "github.com/sirupsen/logrus"
	"github.com/xtls/xray-core/common/protocol"
	"github.com/xtls/xray-core/proxy"
)

func (c *Xray) GetUserManager(tag string) (proxy.UserManager, error) {
	handler, err := c.ihm.GetHandler(context.Background(), tag)
	if err != nil {
		return nil, fmt.Errorf("no such inbound tag: %s", err)
	}
	inboundInstance, ok := handler.(proxy.GetInbound)
	if !ok {
		return nil, fmt.Errorf("handler %s is not implement proxy.GetInbound", tag)
	}
	userManager, ok := inboundInstance.GetInbound().(proxy.UserManager)
	if !ok {
		return nil, fmt.Errorf("handler %s is not implement proxy.UserManager", tag)
	}
	return userManager, nil
}

func (c *Xray) DelUsers(users []panel.UserInfo, tag string, _ *panel.NodeInfo) error {
	if len(users) == 0 {
		return nil
	}

	// Step 1: Get UserManager before locking
	userManager, err := c.GetUserManager(tag)
	if err != nil {
		return fmt.Errorf("get user manager error: %s", err)
	}

	// Step 2: Build user tags and remove from Xray UserManager (without our lock)
	userTags := make([]string, len(users))
	for i := range users {
		userTags[i] = format.UserTag(tag, users[i].Uuid)
		err = userManager.RemoveUser(context.Background(), userTags[i])
		if err != nil {
			return err
		}
	}

	// Step 3: Update our internal maps with minimal lock time
	c.users.mapLock.Lock()
	for _, userTag := range userTags {
		delete(c.users.uidMap, userTag)
	}
	c.users.mapLock.Unlock()

	// Step 4: Clean up counters and link managers (no lock needed, they use sync.Map)
	for _, userTag := range userTags {
		if v, ok := c.dispatcher.Counter.Load(tag); ok {
			tc := v.(*counter.TrafficCounter)
			tc.Delete(userTag)
		}
		if v, ok := c.dispatcher.LinkManagers.Load(userTag); ok {
			lm := v.(*mydispatcher.LinkManager)
			lm.CloseAll()
			c.dispatcher.LinkManagers.Delete(userTag)
		}
	}
	return nil
}

func (x *Xray) GetUserTrafficSlice(tag string, reset bool) ([]panel.UserTraffic, error) {
	trafficSlice := make([]panel.UserTraffic, 0)
	x.users.mapLock.RLock()
	defer x.users.mapLock.RUnlock()
	if v, ok := x.dispatcher.Counter.Load(tag); ok {
		c := v.(*counter.TrafficCounter)
		c.Counters.Range(func(key, value interface{}) bool {
			email := key.(string)
			traffic := value.(*counter.TrafficStorage)
			up := traffic.UpCounter.Load()
			down := traffic.DownCounter.Load()
			if up+down > x.nodeReportMinTrafficBytes[tag] {
				if reset {
					traffic.UpCounter.Store(0)
					traffic.DownCounter.Store(0)
				}
				if x.users.uidMap[email] == 0 {
					c.Delete(email)
					return true
				}
				trafficSlice = append(trafficSlice, panel.UserTraffic{
					UID:      x.users.uidMap[email],
					Upload:   up,
					Download: down,
				})
			}
			return true
		})
		if len(trafficSlice) == 0 {
			return nil, nil
		}
		return trafficSlice, nil
	}
	return nil, nil
}

func (c *Xray) AddUsers(p *vCore.AddUsersParams) (added int, err error) {
	startTotal := time.Now()
	userCount := len(p.Users)

	log.Infof("[%s] Step 1/4: Building %d protocol users...", p.Tag, userCount)
	startBuild := time.Now()

	// Step 1: Build protocol users without lock (CPU-bound, can be done in parallel)
	var users []*protocol.User
	switch p.NodeInfo.Type {
	case "vmess":
		users = buildVmessUsers(p.Tag, p.Users)
	case "vless":
		users = buildVlessUsers(p.Tag, p.Users, p.VAllss.Flow)
	case "trojan":
		users = buildTrojanUsers(p.Tag, p.Users)
	case "shadowsocks":
		users = buildSSUsers(p.Tag,
			p.Users,
			p.Shadowsocks.Cipher,
			p.Shadowsocks.ServerKey)
	default:
		return 0, fmt.Errorf("unsupported node type: %s", p.NodeInfo.Type)
	}
	log.Infof("[%s] Step 1 completed in %v", p.Tag, time.Since(startBuild).Truncate(time.Millisecond))

	// Step 2: Get UserManager before locking
	log.Infof("[%s] Step 2/4: Getting UserManager...", p.Tag)
	man, err := c.GetUserManager(p.Tag)
	if err != nil {
		return 0, fmt.Errorf("get user manager error: %s", err)
	}

	// Step 3: Update uidMap with minimal lock time
	log.Infof("[%s] Step 3/4: Updating uidMap...", p.Tag)
	startMap := time.Now()
	c.users.mapLock.Lock()
	for i := range p.Users {
		c.users.uidMap[format.UserTag(p.Tag, p.Users[i].Uuid)] = p.Users[i].Id
	}
	c.users.mapLock.Unlock()
	log.Infof("[%s] Step 3 completed in %v", p.Tag, time.Since(startMap).Truncate(time.Millisecond))

	// Step 4: Add users to manager without holding our lock (UserManager has its own locking)
	log.Infof("[%s] Step 4/4: Adding %d users to Xray UserManager (this may take a while)...", p.Tag, userCount)
	startAdd := time.Now()
	progressInterval := userCount / 10 // Log every 10%
	if progressInterval < 1000 {
		progressInterval = 1000
	}

	for i, u := range users {
		mUser, err := u.ToMemoryUser()
		if err != nil {
			return 0, err
		}
		err = man.AddUser(context.Background(), mUser)
		if err != nil {
			return 0, fmt.Errorf("failed at user %d/%d: %s", i+1, userCount, err)
		}

		// Progress logging
		if (i+1)%progressInterval == 0 || i+1 == userCount {
			progress := ((i + 1) * 100) / userCount
			elapsed := time.Since(startAdd)
			log.Infof("[%s] Progress: %d%% (%d/%d users, elapsed: %v)",
				p.Tag, progress, i+1, userCount, elapsed.Truncate(time.Second))
		}
	}
	addDuration := time.Since(startAdd)
	log.Infof("[%s] Step 4 completed in %v (%.0f users/sec)",
		p.Tag, addDuration.Truncate(time.Millisecond), float64(userCount)/addDuration.Seconds())

	log.Infof("[%s] Total import completed in %v", p.Tag, time.Since(startTotal).Truncate(time.Millisecond))
	return len(users), nil
}
