package main

import (
	"fmt"
	"log"

	"example.com/salutares"
)

func main() {
	log.SetPrefix("greetings: ")
	log.SetFlags(0)
	message, err := salutares.Ceaw("Pookie coookie")
	if err != nil {
		log.Fatal(err)
	}
	fmt.Println(message)
}
