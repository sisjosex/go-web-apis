package config

var (
	ApplicationPort   string
	ApplicationHost   string
	LanguageDirectory string
	LanguageDefault   string
	DatabaseUrl       string
	AppMode           string
)

func init() {
	ApplicationHost = GetEnv("APP_HOST", "0.0.0.0")
	ApplicationPort = GetEnv("APP_PORT", "8080")
	LanguageDirectory = GetEnv("LANGUAGE_DIRECTORY", "lang")
	LanguageDefault = GetEnv("LANGUAGE_DEFAULT", "en")
	DatabaseUrl = GetEnv("MIGRATE_DATABASE_URL", "postgres://postgres:postgres@localhost:5432/web?sslmode=disable")
	AppMode = GetEnv("GIN_MODE", "release")
}
