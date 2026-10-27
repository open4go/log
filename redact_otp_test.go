package log

import (
	"strings"
	"testing"
)

const fakeOTPURI = "otpauth://totp/Demo:199%2A%2A%2A%2A0000?algorithm=SHA1&digits=6&issuer=Demo&period=30&secret=JBSWY3DPEHPK3PXP"

func TestRedactJSONMasksOTPBindingURI(t *testing.T) {
	body := []byte(`{"step":"mfa-bind","url_otp":"` + fakeOTPURI + `","nested":{"link":"` + fakeOTPURI + `"},"list":["` + fakeOTPURI + `"],"user_id":"u1"}`)
	out, ok := redactJSON(body, false)
	if !ok {
		t.Fatalf("redactJSON failed")
	}
	s := string(out)
	if strings.Contains(s, "JBSWY3DPEHPK3PXP") || strings.Contains(s, "otpauth") {
		t.Fatalf("otp secret leaked: %s", s)
	}
	if !strings.Contains(s, `"user_id":"u1"`) || !strings.Contains(s, `"step":"mfa-bind"`) {
		t.Fatalf("non-sensitive fields must stay: %s", s)
	}
}

func TestRedactJSONTextMasksTruncatedOTPURI(t *testing.T) {
	s := redactJSONText(`{"message":"bind","x":"`+fakeOTPURI, false)
	if strings.Contains(s, "JBSWY3DPEHPK3PXP") {
		t.Fatalf("otp secret leaked in truncated text: %s", s)
	}
}

func TestOTPURLKeysAreSensitive(t *testing.T) {
	for _, k := range []string{"url_otp", "otp_url", "otpauth_url", "otpQrCode"} {
		if !IsSensitiveKey(k, false) {
			t.Errorf("%s should be sensitive", k)
		}
	}
	if IsSensitiveKey("otp_account", false) {
		t.Errorf("otp_account must stay visible")
	}
}

func TestRedactValuesMasksOTPURI(t *testing.T) {
	m := redactValues(map[string][]string{"u": {fakeOTPURI}, "a": {"b"}}, false)
	if m["u"] != RedactedValue || m["a"] != "b" {
		t.Fatalf("unexpected %v", m)
	}
}
