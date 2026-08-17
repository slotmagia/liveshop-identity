package main

import (
	"flag"
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := flag.String("password", "", "password")
	flag.Parse()
	hash, err := bcrypt.GenerateFromPassword([]byte(*password), bcrypt.DefaultCost)
	if err != nil {
		panic(err)
	}
	fmt.Print(string(hash))
}
