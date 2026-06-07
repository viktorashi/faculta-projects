package salutares

import (
	"errors"
	"fmt"
	"math/rand"
)

func Ceaw(name string) (string, error) {
	if name == "" {
		return "", errors.New("Vezi ca n-ai scris nimic")
	}
	return fmt.Sprintf(randomSalutares(), name), nil
}

func CeawLaMulti(names []string) (map[string]string, error) {
	salutations := make(map[string]string)
	for index, name := range names {
		salutares, err := Ceaw(name)
		if err != nil {
			return salutations, errors.New(fmt.Sprintf("Name at index %v is empty, these are the ones handled so far", index))
		}
		salutations[name] = salutares
	}
	return salutations, nil
}

func randomSalutares() string {
	salutations := []string{
		"ke fakii %v",
		"oooh, %v!!!, Supp??",
		"Come here often, %v 😘😘😘😍😍??",
	}
	return salutations[rand.Intn(len(salutations))]
}
