package util

import "fmt"

type ActionableError struct {
	Message string
}

func (e ActionableError) Error() string {
	return fmt.Sprintf("%s", e.Message)
}
