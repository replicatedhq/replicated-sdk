package util

type ActionableError struct {
	Message string
}

func (e ActionableError) Error() string {
	return e.Message
}
