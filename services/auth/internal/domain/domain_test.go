package domain

import (
	"strings"
	"testing"
	"time"
)

func TestLockDuration(t *testing.T) {
	p := DefaultLockout
	cases := []struct {
		failures int
		want     time.Duration
	}{
		{0, 0}, {4, 0}, {5, 30 * time.Second}, {6, time.Minute}, {7, 2 * time.Minute},
		{8, 4 * time.Minute}, {9, 8 * time.Minute}, {10, 15 * time.Minute}, {50, 15 * time.Minute}, {1 << 20, 15 * time.Minute},
	}
	for _, c := range cases {
		if got := p.LockDuration(c.failures); got != c.want {
			t.Errorf("LockDuration(%d) = %v, want %v", c.failures, got, c.want)
		}
	}
}

func TestValidatePassword(t *testing.T) {
	cases := []struct {
		name, pw, email string
		ok              bool
	}{
		{"ok", "correct horse battery", "a@b.kz", true},
		{"short", "short-pass1", "a@b.kz", false},
		{"exactly 12", "abcdefghijkl", "", true},
		{"unicode 12 runes", "пароль-длинн", "", true},
		{"equals email", "Student.One@Example.KZ", "student.one@example.kz", false},
		{"129 runes", strings.Repeat("a", 129), "", false},
		{"128 runes", strings.Repeat("a", 128), "", true},
		{"too many bytes", strings.Repeat("я", 129), "", false},
		{"invalid utf8", "abcdefghijkl\xff", "", false},
	}
	for _, c := range cases {
		var v Validator
		ValidatePassword(&v, c.pw, c.email)
		if (v.Err() == nil) != c.ok {
			t.Errorf("%s: ok=%v err=%v", c.name, c.ok, v.Err())
		}
	}
}

func TestValidateEmailAndText(t *testing.T) {
	for in, ok := range map[string]bool{
		"a@b.kz": true, "": false, "no-at": false, "John <j@x.kz>": false, "a b@c.kz": false,
		strings.Repeat("a", 250) + "@b.kz": false,
	} {
		var v Validator
		ValidateEmail(&v, in)
		if (v.Err() == nil) != ok {
			t.Errorf("email %q ok=%v", in, ok)
		}
	}
	for in, ok := range map[string]bool{"Aitbek K.": true, "": false, " lead": false, "tab\there": false, strings.Repeat("я", 101): false} {
		var v Validator
		ValidateText(&v, "display_name", in, MaxDisplayNameLen)
		if (v.Err() == nil) != ok {
			t.Errorf("text %q ok=%v", in, ok)
		}
	}
}
