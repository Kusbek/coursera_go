package main

import (
	"encoding/json"
	"fmt"
)

func MyPrint(data interface{}) {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		fmt.Println("error:", err)
	}
	fmt.Print(string(b))
	fmt.Println()
}
