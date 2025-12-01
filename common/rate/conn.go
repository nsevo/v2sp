package rate

import (
	"net"

	"github.com/juju/ratelimit"
	"github.com/sagernet/sing/common/buf"
	M "github.com/sagernet/sing/common/metadata"
	"github.com/sagernet/sing/common/network"
)

func NewConnRateLimiter(c net.Conn, l *ratelimit.Bucket) *Conn {
	return &Conn{
		Conn:    c,
		limiter: l,
	}
}

type Conn struct {
	net.Conn
	limiter *ratelimit.Bucket
}

func (c *Conn) Read(b []byte) (n int, err error) {
	c.limiter.Wait(int64(len(b)))
	return c.Conn.Read(b)
}

func (c *Conn) Write(b []byte) (n int, err error) {
	c.limiter.Wait(int64(len(b)))
	return c.Conn.Write(b)
}

type PacketConnLimiter struct {
	network.PacketConn
	limiter *ratelimit.Bucket
}

func NewPacketConnLimiter(conn network.PacketConn, l *ratelimit.Bucket) network.PacketConn {
	if l == nil {
		return conn
	}
	return &PacketConnLimiter{
		PacketConn: conn,
		limiter:    l,
	}
}

func (p *PacketConnLimiter) ReadPacket(buff *buf.Buffer) (destination M.Socksaddr, err error) {
	destination, err = p.PacketConn.ReadPacket(buff)
	if err == nil && buff != nil && buff.Len() > 0 {
		p.limiter.Wait(int64(buff.Len()))
	}
	return destination, err
}

func (p *PacketConnLimiter) WritePacket(buff *buf.Buffer, destination M.Socksaddr) error {
	if buff != nil && buff.Len() > 0 {
		p.limiter.Wait(int64(buff.Len()))
	}
	return p.PacketConn.WritePacket(buff, destination)
}
