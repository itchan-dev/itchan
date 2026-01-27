package main

import (
	"fmt"
	"log"

	"github.com/itchan-dev/itchan/shared/crypto"
)

func main() {
	key, err := crypto.GenerateKey()
	if err != nil {
		log.Fatalf("Failed to generate encryption key: %v", err)
	}

	fmt.Println("=================================================")
	fmt.Println("  Email Encryption Key (AES-256)")
	fmt.Println("=================================================")
	fmt.Println()
	fmt.Println("Generated key (base64):")
	fmt.Println(key)
	fmt.Println()
	fmt.Println("Add this to your config/private.yaml:")
	fmt.Printf("encryption_key: \"%s\"\n", key)
	fmt.Println()
	fmt.Println("IMPORTANT:")
	fmt.Println("- Keep this key secret and secure!")
	fmt.Println("- Back up this key - you cannot decrypt emails without it!")
	fmt.Println("- Never commit this key to version control!")
	fmt.Println("=================================================")
}
