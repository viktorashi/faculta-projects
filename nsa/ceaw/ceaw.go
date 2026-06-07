package main

import (
	"fmt"
	"log"

	"example.com/salutares"
)

func main() {
	log.SetPrefix("greetings: ")
	log.SetFlags(0)
	names := []string{"CEA MAI DESTEATPTA FATA DIN LUMEE", "CEL MAI DESTEATPT baiattt DIN LUMEE"}
	message, err := salutares.CeawLaMulti(names)
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(message)
}
