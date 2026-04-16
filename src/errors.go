package src

import "fmt"

var (
	ErrInvalidUserAgent = fmt.Errorf("user agent cannot be empty")
)

func NewConnectionError(message string) error {
	return fmt.Errorf("connection error: %s", message)
}

func NewInvalidResponseError(message string) error {
	return fmt.Errorf("invalid response: %s", message)
}

func NewInvalidTokenError(message string) error {
	return fmt.Errorf("authentication error: %s", message)
}

func NewTimeoutError(message string) error {
	return fmt.Errorf("timeout error: %s", message)
}
