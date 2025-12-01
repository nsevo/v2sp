package sing

import (
	"context"
	"fmt"
	"io"
	"net"
	"sync"

	"github.com/nsevo/v2sp/common/format"
	"github.com/nsevo/v2sp/common/rate"

	"github.com/nsevo/v2sp/limiter"

	"github.com/nsevo/v2sp/common/counter"
	"github.com/sagernet/sing-box/adapter"
	"github.com/sagernet/sing-box/log"
	N "github.com/sagernet/sing/common/network"
)

var _ adapter.ConnectionTracker = (*HookServer)(nil)

type HookServer struct {
	counter     sync.Map //map[string]*counter.TrafficCounter
	connections sync.Map // key: taguuid, value: *connectionSet
}

func (h *HookServer) ModeList() []string {
	return nil
}

func (h *HookServer) RoutedConnection(_ context.Context, conn net.Conn, m adapter.InboundContext, _ adapter.Rule, _ adapter.Outbound) net.Conn {
	l, err := limiter.GetLimiter(m.Inbound)
	if err != nil {
		log.Warn("get limiter for ", m.Inbound, " error: ", err)
		return conn
	}
	taguuid := format.UserTag(m.Inbound, m.User)
	ip := m.Source.Addr.String()
	if b, r := l.CheckLimit(taguuid, ip, true, true); r {
		conn.Close()
		log.Error("[", m.Inbound, "] ", "Limited ", m.User, " by ip or conn")
		return conn
	} else if b != nil {
		conn = rate.NewConnRateLimiter(conn, b)
	}
	if l != nil {
		destStr := m.Destination.AddrString()
		protocol := m.Protocol
		if l.CheckDomainRule(destStr) {
			log.Error(fmt.Sprintf(
				"User %s access domain %s reject by rule",
				m.User,
				destStr))
			conn.Close()
			return conn
		}
		if len(protocol) != 0 {
			if l.CheckProtocolRule(protocol) {
				log.Error(fmt.Sprintf(
					"User %s access protocol %s reject by rule",
					m.User,
					protocol))
				conn.Close()
				return conn
			}
		}
	}
	var t *counter.TrafficCounter
	if c, ok := h.counter.Load(m.Inbound); !ok {
		t = counter.NewTrafficCounter()
		h.counter.Store(m.Inbound, t)
	} else {
		t = c.(*counter.TrafficCounter)
	}
	conn = counter.NewConnCounter(conn, t.GetCounter(m.User))
	conn = h.trackConn(taguuid, conn)
	return conn
}

func (h *HookServer) RoutedPacketConnection(_ context.Context, conn N.PacketConn, m adapter.InboundContext, _ adapter.Rule, _ adapter.Outbound) N.PacketConn {
	l, err := limiter.GetLimiter(m.Inbound)
	if err != nil {
		log.Warn("get limiter for ", m.Inbound, " error: ", err)
		return conn
	}
	ip := m.Source.Addr.String()
	taguuid := format.UserTag(m.Inbound, m.User)
	if b, r := l.CheckLimit(taguuid, ip, false, true); r {
		conn.Close()
		log.Error("[", m.Inbound, "] ", "Limited ", m.User, " by ip or conn")
		return conn
	} else if b != nil {
		conn = rate.NewPacketConnLimiter(conn, b)
	}
	if l != nil {
		destStr := m.Destination.AddrString()
		protocol := m.Destination.Network()
		if l.CheckDomainRule(destStr) {
			log.Error(fmt.Sprintf(
				"User %s access domain %s reject by rule",
				m.User,
				destStr))
			conn.Close()
			return conn
		}
		if len(protocol) != 0 {
			if l.CheckProtocolRule(protocol) {
				log.Error(fmt.Sprintf(
					"User %s access protocol %s reject by rule",
					m.User,
					protocol))
				conn.Close()
				return conn
			}
		}
	}
	var t *counter.TrafficCounter
	if c, ok := h.counter.Load(m.Inbound); !ok {
		t = counter.NewTrafficCounter()
		h.counter.Store(m.Inbound, t)
	} else {
		t = c.(*counter.TrafficCounter)
	}
	conn = counter.NewPacketConnCounter(conn, t.GetCounter(m.User))
	conn = h.trackPacketConn(taguuid, conn)
	return conn
}

func (h *HookServer) CloseConnections(taguuid string) {
	if v, ok := h.connections.LoadAndDelete(taguuid); ok {
		set := v.(*connectionSet)
		for _, closer := range set.closeAll() {
			_ = closer.Close()
		}
	}
}

func (h *HookServer) trackConn(taguuid string, conn net.Conn) net.Conn {
	tc := &trackedConn{
		Conn:    conn,
		hook:    h,
		taguuid: taguuid,
	}
	h.registerConnection(taguuid, tc)
	return tc
}

func (h *HookServer) trackPacketConn(taguuid string, conn N.PacketConn) N.PacketConn {
	pc := &trackedPacketConn{
		PacketConn: conn,
		hook:       h,
		taguuid:    taguuid,
	}
	h.registerConnection(taguuid, pc)
	return pc
}

func (h *HookServer) registerConnection(taguuid string, closer io.Closer) {
	value, _ := h.connections.LoadOrStore(taguuid, &connectionSet{})
	set := value.(*connectionSet)
	set.add(closer)
}

func (h *HookServer) unregisterConnection(taguuid string, closer io.Closer) {
	if value, ok := h.connections.Load(taguuid); ok {
		set := value.(*connectionSet)
		if set.remove(closer) {
			h.connections.Delete(taguuid)
		}
	}
}

type trackedConn struct {
	net.Conn
	hook    *HookServer
	taguuid string
	once    sync.Once
}

func (c *trackedConn) Close() error {
	c.once.Do(func() {
		c.hook.unregisterConnection(c.taguuid, c)
	})
	return c.Conn.Close()
}

type trackedPacketConn struct {
	N.PacketConn
	hook    *HookServer
	taguuid string
	once    sync.Once
}

func (p *trackedPacketConn) Close() error {
	p.once.Do(func() {
		p.hook.unregisterConnection(p.taguuid, p)
	})
	return p.PacketConn.Close()
}

type connectionSet struct {
	mu      sync.Mutex
	closers map[io.Closer]struct{}
}

func (c *connectionSet) add(closer io.Closer) {
	c.mu.Lock()
	if c.closers == nil {
		c.closers = make(map[io.Closer]struct{})
	}
	c.closers[closer] = struct{}{}
	c.mu.Unlock()
}

func (c *connectionSet) remove(closer io.Closer) bool {
	c.mu.Lock()
	delete(c.closers, closer)
	empty := len(c.closers) == 0
	c.mu.Unlock()
	return empty
}

func (c *connectionSet) closeAll() []io.Closer {
	c.mu.Lock()
	closers := make([]io.Closer, 0, len(c.closers))
	for closer := range c.closers {
		closers = append(closers, closer)
	}
	c.closers = make(map[io.Closer]struct{})
	c.mu.Unlock()
	return closers
}
