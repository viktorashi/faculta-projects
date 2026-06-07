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

func TestCeaw(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		nume    string
		wantErr bool
	}{
		{"nu_vrem_eroare", "pitique", false},
		{"vrem_eroare", "", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := Ceaw(tt.nume)
			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Ceaw() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Ceaw() succeeded unexpectedly")
			}
			want := regexp.MustCompile(`\b` + tt.nume + `\b`)
			if !want.MatchString(got) {
				t.Errorf("Ceaw() = %v, want %v", got, want)
			}
		})
	}
}
