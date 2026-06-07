package salutares

import (
	"errors"
	"fmt"
	"math/rand"
)

func Ceaw(nume string) (string, error) {
	// iti da nume random
	if nume == "" {
		return "", errors.New("Vezi ca n-ai scris nimic")
	}
	return fmt.Sprintf(randomSalutares(), nume), nil
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
