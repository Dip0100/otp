package otp

import (
	"encoding/base32"
	"fmt"
	"testing"
)

func TestValidateOTP(t *testing.T) {
	// Base32 of "12345678901234567890"
	secret, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString("GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ")
	if err != nil {
		t.Fatalf("failed to decode secret: %v", err)
	}

	tests := []struct {
		name    string
		code    string
		counter uint64
		digits  Digits
		algo    Algorithm
		valid   bool
	}{
		{
			name:    "valid HOTP (RFC4226 counter=0)",
			code:    "755224",
			counter: 0,
			digits:  SixDigits,
			algo:    SHA1,
			valid:   true,
		},
		{
			name:    "invalid HOTP (wrong code)",
			code:    "123456",
			counter: 0,
			digits:  SixDigits,
			algo:    SHA1,
			valid:   false,
		},
		{
			name:    "invalid code length",
			code:    "12345", // one digit too short
			counter: 0,
			digits:  SixDigits,
			algo:    SHA1,
			valid:   false,
		},
		{
			name:    "invalid algorithm",
			code:    "000000",
			counter: 0,
			digits:  SixDigits,
			algo:    Algorithm(99),
			valid:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ok, err := validateOTP(tt.code, secret, tt.counter, tt.digits, tt.algo)

			if tt.valid && (!ok || err != nil) {
				t.Errorf("expected valid, got invalid: ok=%v, err=%v", ok, err)
			}

			if !tt.valid && ok {
				t.Errorf("expected invalid, got valid")
			}
		})
	}
}

func TestValidateOTP_RFC4226(t *testing.T) {
	// RFC 4226 Appendix D - Secret (Base32: GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ)
	secretBase32 := "GEZDGNBVGY3TQOJQGEZDGNBVGY3TQOJQ"
	secret, err := DecodeSecret(secretBase32)
	if err != nil {
		t.Fatalf("failed to decode secret: %v", err)
	}

	// Test vectors from RFC 4226 Appendix D
	expected := []string{
		"755224",
		"287082",
		"359152",
		"969429",
		"338314",
		"254676",
		"287922",
		"162583",
		"399871",
		"520489",
	}

	for counter, want := range expected {
		t.Run("RFC4226_Counter_"+string(rune(counter)), func(t *testing.T) {
			ok, err := validateOTP(want, secret, uint64(counter), 6, SHA1)
			if err != nil {
				t.Errorf("unexpected error: %v", err)
			}
			if !ok {
				t.Errorf("expected OTP to be valid at counter %d, got invalid", counter)
			}
		})
	}
}

func TestValidateOTP_RFC6238(t *testing.T) {
	// Test secrets from RFC 6238 Appendix B (in raw binary form)
	secrets := map[Algorithm][]byte{
		SHA1:   []byte("12345678901234567890"),
		SHA256: []byte("12345678901234567890123456789012"),
		SHA512: []byte("1234567890123456789012345678901234567890123456789012345678901234"),
	}

	// Test cases from RFC 6238 Appendix B
	vectors := []struct {
		time    int64
		digits  Digits
		results map[Algorithm]string
	}{
		{59, EightDigits, map[Algorithm]string{SHA1: "94287082", SHA256: "46119246", SHA512: "90693936"}},
		{1111111109, EightDigits, map[Algorithm]string{SHA1: "07081804", SHA256: "68084774", SHA512: "25091201"}},
		{1111111111, EightDigits, map[Algorithm]string{SHA1: "14050471", SHA256: "67062674", SHA512: "99943326"}},
		{1234567890, EightDigits, map[Algorithm]string{SHA1: "89005924", SHA256: "91819424", SHA512: "93441116"}},
		{2000000000, EightDigits, map[Algorithm]string{SHA1: "69279037", SHA256: "90698825", SHA512: "38618901"}},
		{20000000000, EightDigits, map[Algorithm]string{SHA1: "65353130", SHA256: "77737706", SHA512: "47863826"}},
	}

	for _, vec := range vectors {
		counter := uint64(vec.time / 30)
		for algo, expected := range vec.results {
			secret := secrets[algo]
			ok, err := validateOTP(expected, secret, counter, vec.digits, algo)
			if err != nil {
				t.Errorf("unexpected error at time %d with algo %v: %v", vec.time, algo, err)
			}
			if !ok {
				t.Errorf("TOTP RFC6238 failed at time %d with algo %v: got invalid, want %s", vec.time, algo, expected)
			}
		}
	}
}

func BenchmarkValidateOTP(b *testing.B) {
	secrets := map[Algorithm][]byte{
		SHA1:   []byte("12345678901234567890"),
		SHA256: []byte("12345678901234567890123456789012"),
		SHA512: []byte("1234567890123456789012345678901234567890123456789012345678901234"),
	}

	tests := []struct {
		algo   Algorithm
		digits Digits
	}{
		{SHA1, SixDigits},
		{SHA1, EightDigits},
		{SHA1, TenDigits},
		{SHA256, SixDigits},
		{SHA256, EightDigits},
		{SHA256, TenDigits},
		{SHA512, SixDigits},
		{SHA512, EightDigits},
		{SHA512, TenDigits},
	}

	for _, tt := range tests {
		b.Run(fmt.Sprintf("%s/%d", tt.algo.String(), tt.digits), func(b *testing.B) {
			b.ReportAllocs()

			for i := 0; i < b.N; i++ {
				code, err := deriveOTP(secrets[tt.algo], uint64(i), tt.digits.Int(), tt.algo)
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}

				valid, err := validateOTP(code, secrets[tt.algo], uint64(i), tt.digits, tt.algo)
				if err != nil {
					b.Fatalf("unexpected error: %v", err)
				}
				if !valid {
					b.Errorf("expected valid OTP, but got invalid")
				}
			}
		})
	}
}
