package limiter

import (
	"time"

	"github.com/nsevo/v2sp/api/panel"
	"github.com/nsevo/v2sp/common/format"
)

func (l *Limiter) AddDynamicSpeedLimit(tag string, userInfo *panel.UserInfo, limitNum int, expire int64) error {
	userLimit := &UserLimitInfo{
		DynamicSpeedLimit: limitNum,
		ExpireTime:        time.Now().Add(time.Duration(expire) * time.Second).Unix(),
	}
	l.UserLimitInfo.Store(format.UserTag(tag, userInfo.Uuid), userLimit)
	return nil
}

// determineSpeedLimit returns the minimum non-zero rate.
// If one limit is 0 (unlimited), returns the other.
// If both are 0, returns 0 (unlimited).
// If both are non-zero, returns the smaller one.
func determineSpeedLimit(limit1, limit2 int) int {
	if limit1 == 0 {
		return limit2
	}
	if limit2 == 0 {
		return limit1
	}
	if limit1 < limit2 {
		return limit1
	}
	return limit2
}
