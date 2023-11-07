package config

import "josex/web/utils"

var (
	ApplicationPort   string
	ApplicationHost   string
	LanguageDirectory string
	LanguageDefault   string
	DatabaseUrl       string
	AppMode           string
)

func init() {
	ApplicationHost = utils.GetEnv("APP_HOST", "0.0.0.0")
	ApplicationPort = utils.GetEnv("APP_PORT", "8080")
	LanguageDirectory = utils.GetEnv("LANGUAGE_DIRECTORY", "lang")
	LanguageDefault = utils.GetEnv("LANGUAGE_DEFAULT", "en")
	DatabaseUrl = utils.GetEnv("MIGRATE_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/web?sslmode=disable")
	AppMode = utils.GetEnv("GIN_MODE", "release")
}
