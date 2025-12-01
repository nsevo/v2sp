package hy2

import (
	"sync"

	"github.com/apernet/hysteria/core/v2/server"
	quic "github.com/apernet/quic-go"
	"github.com/juju/ratelimit"
	"github.com/nsevo/v2sp/common/counter"
	"github.com/nsevo/v2sp/common/format"
	"github.com/nsevo/v2sp/limiter"
	"go.uber.org/zap"
)

var _ server.TrafficLogger = (*HookServer)(nil)

type HookServer struct {
	Tag                   string
	logger                *zap.Logger
	Counter               sync.Map
	ReportMinTrafficBytes int64
}

func (h *HookServer) TraceStream(stream quic.Stream, stats *server.StreamStats) {
}

func (h *HookServer) UntraceStream(stream quic.Stream) {
}

func (h *HookServer) LogTraffic(id string, tx, rx uint64) (ok bool) {
	var c interface{}
	var exists bool

	limiterinfo, err := limiter.GetLimiter(h.Tag)
	if err != nil {
		h.logger.Error("Get limiter error", zap.String("tag", h.Tag), zap.Error(err))
		return false
	}

	taguuid := format.UserTag(h.Tag, id)
	userLimit, ok := limiterinfo.UserLimitInfo.Load(taguuid)
	if ok {
		userlimitInfo := userLimit.(*limiter.UserLimitInfo)
		if userlimitInfo.OverLimit {
			userlimitInfo.OverLimit = false
			return false
		}
	} else {
		return false
	}

	h.waitSpeedBucket(limiterinfo, taguuid, tx)
	h.waitSpeedBucket(limiterinfo, taguuid, rx)

	if c, exists = h.Counter.Load(h.Tag); !exists {
		c = counter.NewTrafficCounter()
		h.Counter.Store(h.Tag, c)
	}

	if tc, ok := c.(*counter.TrafficCounter); ok {
		tc.Rx(id, int(rx))
		tc.Tx(id, int(tx))
		return true
	}

	return false
}

func (s *HookServer) LogOnlineState(id string, online bool) {
}

func (h *HookServer) waitSpeedBucket(limiterinfo *limiter.Limiter, taguuid string, bytes uint64) {
	if bytes == 0 {
		return
	}
	if bucketVal, ok := limiterinfo.SpeedLimiter.Load(taguuid); ok {
		if bucket, ok := bucketVal.(*ratelimit.Bucket); ok && bucket != nil {
			bucket.Wait(int64(bytes))
		}
	}
}
