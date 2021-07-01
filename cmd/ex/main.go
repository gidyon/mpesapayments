package main

import (
	"fmt"
	"strings"
)

func main() {
	test := strings.ReplaceAll(`test\ntest\ntest\n`, `\n`, "\n")
	fmt.Println(test)
}
