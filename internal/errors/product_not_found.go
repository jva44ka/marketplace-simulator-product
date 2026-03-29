package errors

import (
	errors2 "errors"
	"fmt"
)

type ProductNotFoundError struct {
	Sku uint64
}

func NewProductNotFoundError(sku uint64) *ProductNotFoundError {
	return &ProductNotFoundError{
		Sku: sku,
	}
}

func (e *ProductNotFoundError) Error() string {
	return fmt.Sprintf("%s: sku %d", errors2.New("product not found").Error(), e.Sku)
}

func (e *ProductNotFoundError) Is(target error) bool {
	_, ok := target.(*ProductNotFoundError)

	return ok
}
