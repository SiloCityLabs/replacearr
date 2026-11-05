package cardigann

import (
	"strconv"
	"strings"
	"time"
	"unicode"

	cdefinition "github.com/Kcchouette/cardigann-go/definition"
	"github.com/Kcchouette/cardigann-go/template/filters"
)

// Register Cardigann filters that Prowlarr applies as special-cased field
// parsers but cardigann-go v0.8.0 omits from its filter registry / fieldsToRelease.
func init() {
	filters.Registry["parsebytes"] = parseBytes
	filters.Registry["normalizedate"] = normalizeDate
}

// parseBytes converts human sizes ("1.5 GB", "700MiB", "0 B") and plain byte
// counts into an integer byte string, matching Prowlarr ParseUtil.GetBytes.
func parseBytes(v string, _ []string) (string, error) {
	s := strings.TrimSpace(v)
	if s == "" {
		return "0", nil
	}
	// Strip thousands separators used by some trackers.
	s = strings.ReplaceAll(s, ",", "")

	var numBuf strings.Builder
	var unitBuf strings.Builder
	seenDot := false
	for _, r := range s {
		switch {
		case unicode.IsDigit(r):
			numBuf.WriteRune(r)
		case r == '.' && !seenDot:
			seenDot = true
			numBuf.WriteRune(r)
		case unicode.IsLetter(r):
			unitBuf.WriteRune(unicode.ToLower(r))
		}
	}
	numStr := numBuf.String()
	if numStr == "" || numStr == "." {
		return "0", nil
	}
	val, err := strconv.ParseFloat(numStr, 64)
	if err != nil {
		return "0", nil
	}

	unit := strings.ReplaceAll(unitBuf.String(), "i", "")
	var bytes float64
	switch {
	case strings.Contains(unit, "tb"):
		bytes = val * 1024 * 1024 * 1024 * 1024
	case strings.Contains(unit, "gb"):
		bytes = val * 1024 * 1024 * 1024
	case strings.Contains(unit, "mb"):
		bytes = val * 1024 * 1024
	case strings.Contains(unit, "kb"):
		bytes = val * 1024
	default:
		// No unit (or bare "b" / "byte") — already bytes.
		bytes = val
	}
	if bytes < 0 {
		bytes = 0
	}
	return strconv.FormatInt(int64(bytes), 10), nil
}

// normalizeDate accepts values Prowlarr DateTimeUtil.FromUnknown handles that
// cardigann-go's fieldsToRelease does not: unix seconds/millis, "now", and
// RFC3339 with fractional seconds. Already-RFC3339 values pass through.
func normalizeDate(v string, _ []string) (string, error) {
	s := strings.TrimSpace(v)
	if s == "" {
		return "", nil
	}
	lower := strings.ToLower(s)
	if lower == "now" {
		return time.Now().UTC().Format(time.RFC3339), nil
	}
	if isAllDigits(s) {
		n, err := strconv.ParseInt(s, 10, 64)
		if err != nil {
			return s, nil
		}
		// Heuristic: 13+ digit timestamps are milliseconds.
		if n > 1_000_000_000_000 {
			n = n / 1000
		}
		if n <= 0 {
			return s, nil
		}
		return time.Unix(n, 0).UTC().Format(time.RFC3339), nil
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC().Format(time.RFC3339), nil
	}
	// Leave other layouts for prior filters (dateparse/fuzzytime/timeago).
	return s, nil
}

func isAllDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func hasFilterNamed(steps []cdefinition.FilterStep, name string) bool {
	for _, step := range steps {
		if strings.EqualFold(step.Name, name) {
			return true
		}
	}
	return false
}

func appendFilter(steps []cdefinition.FilterStep, name string) []cdefinition.FilterStep {
	if hasFilterNamed(steps, name) {
		return steps
	}
	return append(steps, cdefinition.FilterStep{Name: name})
}
