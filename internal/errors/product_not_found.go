package errors

import (
	"errors"
	"fmt"
)

const productNotFoundErrorText = "product not found"

type ProductNotFoundError struct {
	Sku uint64
}

func NewProductNotFoundError(sku uint64) *ProductNotFoundError {
	return &ProductNotFoundError{
		Sku: sku,
	}
}

func (e *ProductNotFoundError) Error() string {
	return fmt.Sprintf("%s: sku %d", errors.New(productNotFoundErrorText).Error(), e.Sku)
}

func (e *ProductNotFoundError) Is(target error) bool {
	_, ok := target.(*ProductNotFoundError)

	return ok
}
