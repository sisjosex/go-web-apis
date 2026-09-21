package jobs

import (
	"net/http"

	coreServices "josex/web/modules/core/services"

	"github.com/hibiken/asynq"
	"github.com/hibiken/asynqmon"
)

// MonitorPath is where asynqmon is mounted, behind platform-admin auth.
const MonitorPath = "/admin/jobs"

var _ asynq.RedisConnOpt = sharedConn{}

// sharedConn hands asynqmon the process's client instead of letting it open a second pool.
type sharedConn struct {
	valkey coreServices.ValkeyService
}

func (s sharedConn) MakeRedisClient() interface{} {
	return s.valkey.Client()
}

// Monitor returns the asynqmon UI and API rooted at MonitorPath.
func Monitor(valkey coreServices.ValkeyService) http.Handler {
	return asynqmon.New(asynqmon.Options{
		RootPath:     MonitorPath,
		RedisConnOpt: sharedConn{valkey: valkey},
	})
}
