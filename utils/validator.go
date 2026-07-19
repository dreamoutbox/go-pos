package utils

import (
	"fmt"

	"github.com/go-playground/validator/v10"
)

var Validate *validator.Validate

func InitValidator() {
	Validate = validator.New()
}

// FormatValidationError formats validation errors into human-readable field messages
func FormatValidationError(err error) map[string]string {
	errors := make(map[string]string)
	if err == nil {
		return errors
	}

	validationErrors, ok := err.(validator.ValidationErrors)
	if !ok {
		errors["_global"] = err.Error()
		return errors
	}

	for _, fieldErr := range validationErrors {
		fieldName := fieldErr.Field()
		tag := fieldErr.Tag()
		param := fieldErr.Param()

		switch tag {
		case "required":
			errors[fieldName] = fmt.Sprintf("The %s field is required.", fieldName)
		case "email":
			errors[fieldName] = "Please enter a valid email address."
		case "min":
			errors[fieldName] = fmt.Sprintf("The %s field must be at least %s characters long.", fieldName, param)
		case "gt":
			errors[fieldName] = fmt.Sprintf("The %s field must be greater than %s.", fieldName, param)
		case "gte":
			errors[fieldName] = fmt.Sprintf("The %s field must be greater than or equal to %s.", fieldName, param)
		case "oneof":
			errors[fieldName] = fmt.Sprintf("The %s field must be one of: %s.", fieldName, param)
		default:
			errors[fieldName] = fmt.Sprintf("The %s field failed validation on '%s'.", fieldName, tag)
		}
	}
	return errors
}
