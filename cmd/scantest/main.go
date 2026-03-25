package main

import (
	"encoding/json"
	"fmt"
	"os"
	"tabmate/internals/menu"

	"github.com/joho/godotenv"
)

func main() {
	_ = godotenv.Load()

	if len(os.Args) < 2 {
		fmt.Println("usage: go run cmd/scantest/main.go <image_path>")
		os.Exit(1)
	}

	imageBytes, err := os.ReadFile(os.Args[1])
	if err != nil {
		fmt.Printf("failed to read file: %v\n", err)
		os.Exit(1)
	}

	items, err := menu.ScanMenuImage(imageBytes, "image/jpeg")
	if err != nil {
		fmt.Printf("scan error: %v\n", err)
		os.Exit(1)
	}

	out, _ := json.MarshalIndent(items, "", "  ")
	fmt.Println(string(out))
}
