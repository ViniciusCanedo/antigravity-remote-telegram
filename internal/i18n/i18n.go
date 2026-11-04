package i18n

import (
	"embed"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"antigravity-telegram-remote/internal/logger"
)

// Language represents the supported language codes ('en', 'ja', 'vi').
type Language string

const (
	LangEN Language = "en"
	LangJA Language = "ja"
	LangVI Language = "vi"
)

//go:embed locales/*.json
var embeddedLocales embed.FS

// Translator manages thread-safe multi-language translations and variable interpolation.
type Translator struct {
	mu              sync.RWMutex
	currentLanguage Language
	translations    map[Language]map[string]string
	readFileFunc    func(lang Language) ([]byte, error)
}

func defaultFileReader(lang Language) ([]byte, error) {
	return embeddedLocales.ReadFile("locales/" + string(lang) + ".json")
}

func (t *Translator) loadTranslations() {
	langs := []Language{LangEN, LangJA, LangVI}
	for _, lang := range langs {
		data, err := t.readFileFunc(lang)
		if err != nil {
			t.translations[lang] = make(map[string]string)
			continue
		}
		var dict map[string]string
		if err := json.Unmarshal(data, &dict); err != nil {
			logger.Error("Failed to load translations:", err)
			t.translations[lang] = make(map[string]string)
			continue
		}
		t.translations[lang] = dict
	}
}

// NewTranslator creates a Translator initialized with default embedded locales.
func NewTranslator() *Translator {
	t := &Translator{
		currentLanguage: LangVI,
		translations:    make(map[Language]map[string]string),
		readFileFunc:    defaultFileReader,
	}
	t.loadTranslations()
	return t
}

var defaultTranslator = NewTranslator()

// InitI18n initializes the global translator with an optional language (defaults to LangVI).
func (t *Translator) InitI18n(lang ...Language) {
	t.mu.Lock()
	defer t.mu.Unlock()

	targetLang := LangVI
	if len(lang) > 0 && lang[0] != "" {
		targetLang = lang[0]
	}
	t.currentLanguage = targetLang
	t.readFileFunc = defaultFileReader
	t.loadTranslations()
}

// InitI18nWithReader initializes the translator with a custom reader for testing.
func (t *Translator) InitI18nWithReader(lang Language, reader func(lang Language) ([]byte, error)) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.currentLanguage = lang
	t.readFileFunc = reader
	t.loadTranslations()
}

// T translates a key using the active language with fallback to English and then to the raw key,
// substituting double-brace {{variable}} placeholders with literal values.
func (t *Translator) T(key string, variables ...map[string]any) string {
	if key == "" {
		return ""
	}
	t.mu.RLock()
	defer t.mu.RUnlock()

	langDict := t.translations[t.currentLanguage]
	if langDict == nil {
		langDict = t.translations[LangEN]
	}

	text := ""
	if langDict != nil {
		text = langDict[key]
	}
	if text == "" {
		if enDict := t.translations[LangEN]; enDict != nil {
			text = enDict[key]
		}
	}
	if text == "" {
		text = key
	}

	for _, vars := range variables {
		for vKey, vValue := range vars {
			var strVal string
			if vValue == nil {
				strVal = "null"
			} else {
				strVal = fmt.Sprintf("%v", vValue)
			}
			text = strings.ReplaceAll(text, "{{"+vKey+"}}", strVal)
		}
	}

	return text
}

// GetCurrentLanguage returns the currently active language.
func (t *Translator) GetCurrentLanguage() Language {
	t.mu.RLock()
	defer t.mu.RUnlock()
	return t.currentLanguage
}

// Global package delegates:
func InitI18n(lang ...Language) {
	defaultTranslator.InitI18n(lang...)
}

func InitI18nWithReader(lang Language, reader func(lang Language) ([]byte, error)) {
	defaultTranslator.InitI18nWithReader(lang, reader)
}

func T(key string, variables ...map[string]any) string {
	return defaultTranslator.T(key, variables...)
}

func GetCurrentLanguage() Language {
	return defaultTranslator.GetCurrentLanguage()
}
