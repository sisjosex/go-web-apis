//go:build noasynqmon

package jobs

import (
	"net/http"

	coreServices "josex/web/modules/core/services"
)

// MonitorPath is where asynqmon would be mounted: see monitor.go.
const MonitorPath = "/admin/jobs"

// Monitor is nil in this build: nothing is mounted.
func Monitor(coreServices.ValkeyService) http.Handler { return nil }
