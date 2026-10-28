package log

import "unicode"

// MaskPhone returns a phone number with the middle digits replaced by ****.
// For 11+ digit numbers the last 11 digits are treated as the national number
// (e.g. CN mobile): "19912341002" → "199****1002", "+8619912341002" → "+86199****1002".
// Shorter values become "***". Empty stays empty.
func MaskPhone(phone string) string {
	if phone == "" {
		return ""
	}
	runes := []rune(phone)
	digitIdx := make([]int, 0, len(runes))
	for i, r := range runes {
		if unicode.IsDigit(r) {
			digitIdx = append(digitIdx, i)
		}
	}
	n := len(digitIdx)
	if n < 7 {
		return RedactedValue
	}
	// Prefer masking within the trailing 11-digit national number when present.
	start := 0
	if n >= 11 {
		start = n - 11
	}
	national := digitIdx[start:]
	nn := len(national)
	keepHead, keepTail := 3, 4
	for i := keepHead; i < nn-keepTail; i++ {
		runes[national[i]] = '*'
	}
	return string(runes)
}
