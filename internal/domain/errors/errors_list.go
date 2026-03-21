package errors

import "errors"

var (
	ErrProductNotFound     = errors.New("product not found")
	ErrInsufficientProduct = errors.New("insufficient product")
)
