package httpd

import "github.com/go-playground/validator/v10"

type requestValidator struct {
	validate *validator.Validate
}
