package salutares

import (
	"errors"
	"fmt"
)

func Ceaw(name string) (string, error) {
	if name == "" {
		return "", errors.New("Vezi ca n-ai scris nimic")
	}
	return fmt.Sprintf("ce faki %v", name), nil
}
