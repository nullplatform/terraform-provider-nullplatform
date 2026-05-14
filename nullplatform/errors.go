package nullplatform

import (
	"errors"
	"fmt"
)

type ApiError struct {
	ID      int    `json:"id"`
	Message string `json:"message"`
}

func (e *ApiError) Error() string {
	return fmt.Sprintf("%d: %s", e.ID, e.Message)
}

type HTTPStatusError struct {
	Status  int
	Message string
}

func (e *HTTPStatusError) Error() string {
	return fmt.Sprintf("status code: %d, message: %s", e.Status, e.Message)
}

func (e *HTTPStatusError) StatusCode() int {
	return e.Status
}

type ResourceExistsError struct {
	ApiType string
	ID      int
	Message string
}

func (e *ResourceExistsError) Error() string {
	return fmt.Sprintf("%s already exists (%d): %s", e.ApiType, e.ID, e.Message)
}

func IsResourceExistsError(err error) (*ResourceExistsError, bool) {
	if err == nil {
		return nil, false
	}

	var resourceExistsError *ResourceExistsError
	ok := errors.As(err, &resourceExistsError)

	return resourceExistsError, ok
}

type ResourceNotFoundError struct {
	ApiType string
	ID      int
	Message string
}

func (e *ResourceNotFoundError) Error() string {
	return fmt.Sprintf("%s not found (%d): %s", e.ApiType, e.ID, e.Message)
}

func IsResourceNotFoundError(err error) (*ResourceNotFoundError, bool) {
	if err == nil {
		return nil, false
	}

	var resourceNotFoundError *ResourceNotFoundError
	ok := errors.As(err, &resourceNotFoundError)

	return resourceNotFoundError, ok
}
