package interfaces

import (
	"net/http"
)

type Route struct {
	Method  string
	Path    string
	Handler func(w http.ResponseWriter, r *http.Request)
}

type WebServerService interface {
	Initialize(dbService *DatabaseService)
	Start()
}
