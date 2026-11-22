package util

import (
	"regexp"
	"strings"
)

var (
	bearerRe = regexp.MustCompile(`(?i)(Bearer\s+)[A-Za-z0-9._~-]+`)
	skRe     = regexp.MustCompile(`sk-[A-Za-z0-9_-]{8,}`)
)

// Redact removes known secrets and common key patterns before anything is
// logged or stored. Error bodies from LLM providers are never shown to users.
func Redact(s string, secrets []string) string {
	for _, secret := range secrets {
		secret = strings.TrimSpace(secret)
		if len(secret) >= 6 {
			s = strings.ReplaceAll(s, secret, "***")
		}
	}
	s = bearerRe.ReplaceAllString(s, "${1}***")
	s = skRe.ReplaceAllString(s, "sk-***")
	r := []rune(s)
	if len(r) > 500 {
		r = r[:500]
	}
	return strings.Join(strings.Fields(string(r)), " ")
}
