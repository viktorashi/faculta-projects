package salutares

import (
	"errors"
	"fmt"
	"math/rand"
)

func Ceaw(name string) (string, error) {
	return handleName(string)
}

func CeawLaMulti(names [string]) []string, error {
	var res []string, error
	for name in names{
		res.append(name)
	}
	return res
}

func handleName()(name string) (string, error) {
		if name == "" {
		return "", errors.New("Vezi ca n-ai scris nimic")
	}
	return fmt.Sprintf(randomSalutares(), name), nil

}

func randomSalutares() string {
	salutations := []string{
		"ke fakii %v",
		"oooh, %v!!!, Supp??",
		"Come here often, %v 😘😘😘😍😍??",
	}
	return salutations[rand.Intn(len(salutations))]
}
