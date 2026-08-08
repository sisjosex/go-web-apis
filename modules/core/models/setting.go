package models

// Setting is one configuration value from core.settings, addressed as
// <module>.<section>.<key> (e.g. import / images / max_zip_mb).
type Setting struct {
	Module  string `json:"module"`
	Section string `json:"section"`
	Key     string `json:"key"`
	Value   string `json:"value"`
}
