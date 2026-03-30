package errors

import (
	"errors"
	"fmt"
)

const insufficientProductErrorText = "insufficient product"

type InsufficientProductError struct {
	Sku       uint64
	HaveCount uint32
	WantCount uint32
}

func NewInsufficientProductError(sku uint64, haveCount, wantCount uint32) *InsufficientProductError {
	return &InsufficientProductError{
		Sku:       sku,
		HaveCount: haveCount,
		WantCount: wantCount,
	}
}

func (e *InsufficientProductError) Error() string {
	return fmt.Sprintf("%s: sku %d, have %d, want %d",
		errors.New(insufficientProductErrorText), e.Sku, e.WantCount, e.WantCount)
}

func (e *InsufficientProductError) Is(target error) bool {
	_, ok := target.(*InsufficientProductError)

	return ok
}
