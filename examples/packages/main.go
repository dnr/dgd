package main

import (
	"fmt"

	"example.com/dgd-examples/hello/a"
	"example.com/dgd-examples/hello/b"
)

func main() {
	fmt.Println(a.A() + " " + b.B() + "!")
}
