package recon

import "strings"

const minDomainLabels = 2

func validateCandidates(candidates map[string]candidateMeta) map[string]candidateMeta {
	valid := make(map[string]candidateMeta, len(candidates))
	for domain, meta := range candidates {
		if !IsValidDomain(domain) {
			continue
		}
		valid[domain] = meta
	}
	return valid
}

func IsValidDomain(domain string) bool {
	domain = normalizeDomain(domain)
	if !isValidDomainLength(domain) {
		return false
	}

	parts, ok := domainParts(domain)
	if !ok {
		return false
	}
	return allPartsValid(parts)
}

func isValidDomainLength(domain string) bool {
	return domain != "" && len(domain) <= 253
}

func domainParts(domain string) ([]string, bool) {
	parts := strings.Split(domain, ".")
	if len(parts) < minDomainLabels {
		return nil, false
	}
	return parts, true
}

func allPartsValid(parts []string) bool {
	for i, part := range parts {
		if !isValidDomainPart(part, i == len(parts)-1) {
			return false
		}
	}
	return true
}

func isValidDomainPart(part string, isTLD bool) bool {
	if part == "" || len(part) > 63 {
		return false
	}
	if strings.HasPrefix(part, "-") || strings.HasSuffix(part, "-") {
		return false
	}
	if hasHyphen(part) {
		return false
	}

	for _, r := range part {
		if !isLowerAlphaNum(r) {
			return false
		}
	}

	if isTLD {
		return hasValidTLDLength(part)
	}
	return true
}

func hasValidTLDLength(part string) bool {
	return len(part) >= 2
}

func hasHyphen(part string) bool {
	return strings.IndexByte(part, '-') >= 0
}

func isLowerAlphaNum(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9')
}
