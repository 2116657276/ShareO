package main

import (
	"fmt"
	"os"

	"golang.org/x/crypto/bcrypt"
)

func main() {
	password := os.Getenv("SHAREO_GENHASH_PASSWORD")
	if len(os.Args) >= 2 {
		password = os.Args[1]
	}
	if password == "" {
		fmt.Fprintf(os.Stderr, "Usage: genhash <password> or SHAREO_GENHASH_PASSWORD=<password>\n")
		os.Exit(1)
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}
	fmt.Println(string(hash))
}
