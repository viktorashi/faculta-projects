package salutares

import (
	"regexp"
	"testing"
)

func TestHelloName(t *testing.T) {
	name := "pitique"
	want := regexp.MustCompile(`\b` + name + `\b`)
	msg, err := Ceaw(name)
	if !want.MatchString(msg) || err != nil {
		t.Errorf(`Ceaw(%v) = (%v, %v), dar vrem (%#q, nil)`, name, msg, err, want)
	}
}

func TestHelloEmpty(t *testing.T) {
	name := ""
	msg, err := Ceaw(name)
	if msg != "" || err == nil {
		t.Errorf(`Ceaw(%v) = (%v, %v), dar vrem ("", error)`, name, msg, err)
	}
}
