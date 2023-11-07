package services

import (
	"sync"
)

// Define una estructura para los mensajes de error en diferentes idiomas
type ErrorMessages map[string]map[string]string

// Define una estructura para el archivo de idioma
type LanguageFile struct {
	Messages ErrorMessages
}

type LanguageService struct {
	languageCache map[string]*LanguageFile
	mu            sync.RWMutex
}

func NewLanguageService() *LanguageService {
	return &LanguageService{
		languageCache: make(map[string]*LanguageFile),
	}
}
