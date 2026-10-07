package redaction

import "regexp"

var (
	secretPattern         = regexp.MustCompile(`(?i)\b(?:sk|rk|pk)-[A-Za-z0-9_-]{12,}\b|\bBearer\s+[A-Za-z0-9._-]{12,}`)
	emailPattern          = regexp.MustCompile(`\b[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}\b`)
	keyValueSecretPattern = regexp.MustCompile(`(?i)\b([a-z0-9_-]*(?:token|secret|password|passwd|api_key|apikey|authorization|credential)[a-z0-9_-]*)(\s*[:=]\s*)(?:"[^"]*"|'[^']*'|[^\s,;]+)`)

	secretMarker = "[REDACTED_SECRET]"
	emailMarker  = "[REDACTED_EMAIL]"
	valueMarker  = "[REDACTED]"
)

// Redact removes common credential and email forms from s using the
// experimental redaction markers.
func Redact(s string) string {
	s = secretPattern.ReplaceAllString(s, secretMarker)
	s = keyValueSecretPattern.ReplaceAllString(s, `${1}${2}`+valueMarker)
	return emailPattern.ReplaceAllString(s, emailMarker)
}
