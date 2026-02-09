package utils

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

var validate = validator.New()

// ValidateStruct validates a struct based on `validate` tags.
func ValidateStruct(v any) error {
	return validate.Struct(v)
}

// ValidationErrorMessage returns a short, user-friendly message.
func ValidationErrorMessage(err error) string {
	if ve, ok := err.(validator.ValidationErrors); ok && len(ve) > 0 {
		fe := ve[0]
		return fmt.Sprintf("invalid %s", fe.Field())
	}
	return "invalid input"
}
