// Package i18n is a stub; translation is dropped. Always returns "en".
package i18n

// Locale returns the always-fixed locale string.
func Locale() string { return "en" }

// T returns the string as-is (translation no-op).
func T(s string) string { return s }
