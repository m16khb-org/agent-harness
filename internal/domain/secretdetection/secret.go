package secretdetection

import "regexp"

var patterns = []*regexp.Regexp{
	regexp.MustCompile(`(?i)(api[_-]?key|apikey|secret|token|password|passwd|credential|private[_-]?key)\s*[:=]\s*\S+`),
	regexp.MustCompile(`-----BEGIN (RSA |EC |DSA |OPENSSH |PGP )?PRIVATE KEY-----`),
	regexp.MustCompile(`(?i)(ghp|gho|ghu|ghs|ghr|glpat|gldt|glft)_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`eyJ[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{20,}\.[A-Za-z0-9_-]{10,}`),
}

func Contains(value string) bool {
	for _, pattern := range patterns {
		if pattern.MatchString(value) {
			return true
		}
	}
	return false
}
