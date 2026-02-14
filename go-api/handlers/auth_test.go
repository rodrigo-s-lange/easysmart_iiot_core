package handlers

import "testing"

func TestIsValidEmail(t *testing.T) {
	t.Parallel()

	tests := []struct {
		email string
		want  bool
	}{
		{email: "user@example.com", want: true},
		{email: "cliente+1@example.com", want: true},
		{email: "invalid-email", want: false},
		{email: "no-domain@", want: false},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.email, func(t *testing.T) {
			t.Parallel()
			got := isValidEmail(tt.email)
			if got != tt.want {
				t.Fatalf("isValidEmail(%q) = %v, want %v", tt.email, got, tt.want)
			}
		})
	}
}

func TestValidatePassword(t *testing.T) {
	t.Parallel()

	valid := "Abcdef1!"
	if err := validatePassword(valid); err != nil {
		t.Fatalf("validatePassword(%q) unexpected error: %v", valid, err)
	}

	invalid := []string{
		"short1!",   // too short
		"abcdef1!",  // no upper
		"ABCDEF1!",  // no lower
		"Abcdefgh!", // no number
		"Abcdef12",  // no special
	}

	for _, pwd := range invalid {
		pwd := pwd
		t.Run(pwd, func(t *testing.T) {
			t.Parallel()
			if err := validatePassword(pwd); err == nil {
				t.Fatalf("validatePassword(%q) expected error, got nil", pwd)
			}
		})
	}
}
