package log

import (
	"bytes"
	"encoding/json"
	"net/url"
	"regexp"
	"strings"
	"sync"
)

// RedactedValue replaces the value of every sensitive key in access logs.
const RedactedValue = "***"

// sensitiveExact are normalized key names (lower case, without _ - .)
// whose values are never written to the log.
var sensitiveExact = map[string]struct{}{
	"password": {}, "passwd": {}, "pwd": {}, "pass": {}, "passcode": {},
	"token": {}, "accesstoken": {}, "refreshtoken": {}, "idtoken": {}, "jwt": {},
	"sessionid": {},
	"secret":    {}, "appsecret": {}, "clientsecret": {}, "secretkey": {}, "apisecret": {},
	"key": {}, "privatekey": {}, "apikey": {}, "apiv3key": {}, "apiv2key": {}, "mchkey": {},
	"aeskey": {}, "encryptkey": {}, "signkey": {},
	"authorization": {}, "cookie": {}, "setcookie": {},
	"otp": {}, "totp": {}, "otpcode": {}, "otpsecret": {}, "mfacode": {},
	"urlotp": {}, "otpurl": {}, "otpuri": {}, "otpauth": {}, "otpauthurl": {}, "otpqr": {}, "otpqrcode": {}, "mfaurl": {},
	"smscode": {}, "verifycode": {}, "verificationcode": {}, "captcha": {}, "authcode": {},
	"idcard": {}, "idcardno": {}, "idcardnumber": {}, "idno": {}, "identitycard": {}, "identityno": {},
	"cardno": {}, "cardnumber": {}, "bankcard": {}, "bankcardno": {}, "bankaccount": {},
	"cvv": {}, "cvc": {}, "pin": {},
}

// sensitiveSuffix catch compound names such as newPassword, jwtToken,
// wxAppSecret or merchantPrivateKey.
var sensitiveSuffix = []string{
	"password", "passwd", "secret", "token", "privatekey", "apikey", "secretkey", "idcard", "cardno",
}

// requestOnlyExact are only sensitive in request payloads. Responses use
// "code" for the business status code, which must stay visible.
var requestOnlyExact = map[string]struct{}{"code": {}}

var (
	extraMu        sync.RWMutex
	extraSensitive = map[string]struct{}{}
)

// AddSensitiveKeys registers more key names to redact in access logs.
func AddSensitiveKeys(keys ...string) {
	extraMu.Lock()
	defer extraMu.Unlock()
	for _, k := range keys {
		if n := normalizeKey(k); n != "" {
			extraSensitive[n] = struct{}{}
		}
	}
}

func normalizeKey(k string) string {
	k = strings.ToLower(strings.TrimSpace(k))
	return strings.NewReplacer("_", "", "-", "", ".", "", " ", "").Replace(k)
}

// IsSensitiveKey reports whether values under key are redacted.
// request=true also treats request-only names (such as an SMS "code") as sensitive.
func IsSensitiveKey(key string, request bool) bool {
	n := normalizeKey(key)
	if n == "" {
		return false
	}
	if _, ok := sensitiveExact[n]; ok {
		return true
	}
	if request {
		if _, ok := requestOnlyExact[n]; ok {
			return true
		}
	}
	for _, s := range sensitiveSuffix {
		if strings.HasSuffix(n, s) {
			return true
		}
	}
	extraMu.RLock()
	_, ok := extraSensitive[n]
	extraMu.RUnlock()
	return ok
}

// redactValue walks decoded JSON and masks sensitive keys at any depth.
func redactValue(v any, request bool) any {
	switch x := v.(type) {
	case map[string]any:
		for k, val := range x {
			if IsSensitiveKey(k, request) {
				if val != nil {
					x[k] = RedactedValue
				}
				continue
			}
			x[k] = redactValue(val, request)
		}
		return x
	case []any:
		for i := range x {
			x[i] = redactValue(x[i], request)
		}
		return x
	case string:
		if hasSecretURI(x) {
			return RedactedValue
		}
		return v
	default:
		return v
	}
}

// secretURIRe matches provisioning URIs that embed a shared secret
// (otpauth:// TOTP/HOTP binding links), whatever key they are stored under.
var secretURIRe = regexp.MustCompile(`(?i)otpauth(?:-migration)?:(?://|%2F%2F)[^"\s]*`)

func hasSecretURI(s string) bool {
	return strings.Contains(strings.ToLower(s), "otpauth") && secretURIRe.MatchString(s)
}

// redactJSON returns a compact, redacted copy of a complete JSON document.
func redactJSON(raw []byte, request bool) (json.RawMessage, bool) {
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.UseNumber()
	var v any
	if err := dec.Decode(&v); err != nil {
		return nil, false
	}
	if dec.More() {
		return nil, false
	}
	var out bytes.Buffer
	enc := json.NewEncoder(&out)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(redactValue(v, request)); err != nil {
		return nil, false
	}
	return json.RawMessage(bytes.TrimRight(out.Bytes(), "\n")), true
}

// jsonPairRe matches `"key": value` in (possibly truncated) JSON text.
var jsonPairRe = regexp.MustCompile(`"((?:[^"\\]|\\.){1,128})"\s*:\s*("(?:[^"\\]|\\.)*"?|[^,{}\[\]\s"]+)`)

// redactJSONText masks sensitive keys in JSON text that could not be
// parsed (for example because it was truncated).
func redactJSONText(s string, request bool) string {
	s = secretURIRe.ReplaceAllString(s, RedactedValue)
	return jsonPairRe.ReplaceAllStringFunc(s, func(m string) string {
		sub := jsonPairRe.FindStringSubmatch(m)
		if len(sub) < 3 || !IsSensitiveKey(sub[1], request) {
			return m
		}
		return `"` + sub[1] + `":"` + RedactedValue + `"`
	})
}

// redactValues turns url.Values into a JSON-friendly map with sensitive
// keys masked. Single values become strings, repeated ones arrays.
func redactValues(vals url.Values, request bool) map[string]any {
	if len(vals) == 0 {
		return nil
	}
	out := make(map[string]any, len(vals))
	for k, vs := range vals {
		if IsSensitiveKey(k, request) {
			out[k] = RedactedValue
			continue
		}
		if len(vs) == 1 {
			if hasSecretURI(vs[0]) {
				out[k] = RedactedValue
				continue
			}
			out[k] = vs[0]
			continue
		}
		cp := make([]string, len(vs))
		copy(cp, vs)
		out[k] = cp
	}
	return out
}
