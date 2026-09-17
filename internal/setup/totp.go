package setup

import (
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	qrcode "github.com/skip2/go-qrcode"
)

func NewTOTPSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

func Code(secret string, t time.Time) (string, error) {
	sec := strings.ToUpper(strings.TrimSpace(secret))
	key, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(sec)
	if err != nil {
		return "", err
	}
	counter := uint64(t.Unix() / 30)
	var buf [8]byte
	binary.BigEndian.PutUint64(buf[:], counter)
	mac := hmac.New(sha1.New, key)
	_, _ = mac.Write(buf[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	bin := binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff
	return fmt.Sprintf("%06d", bin%1000000), nil
}

func VerifyTOTP(secret, code string, now time.Time) bool {
	code = strings.TrimSpace(code)
	if _, err := strconv.Atoi(code); err != nil || len(code) != 6 {
		return false
	}
	for _, d := range []time.Duration{0, -30 * time.Second, 30 * time.Second} {
		want, err := Code(secret, now.Add(d))
		if err == nil && hmac.Equal([]byte(want), []byte(code)) {
			return true
		}
	}
	return false
}

func OTPAuthURL(user, secret string) string {
	user = strings.TrimSpace(user)
	secret = strings.TrimSpace(secret)
	q := url.Values{}
	q.Set("secret", secret)
	q.Set("issuer", "K-CRM")
	q.Set("digits", "6")
	q.Set("period", "30")
	return "otpauth://totp/" + url.PathEscape("K-CRM") + ":" + url.PathEscape(user) + "?" + q.Encode()
}

func TOTPQR(user, secret string) (otpauth, qr string, err error) {
	otpauth = OTPAuthURL(user, secret)
	png, err := qrcode.Encode(otpauth, qrcode.Medium, 256)
	if err != nil {
		return otpauth, "", err
	}
	return otpauth, "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}
