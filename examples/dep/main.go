package main

import (
	"fmt"

	"github.com/frioux/shellquote"
	"github.com/google/shlex"
)

func main() {
	args, err := shlex.Split("Hello,     DGD!")
	if err != nil {
		panic(err)
	}
	q, err := shellquote.Quote(args)
	if err != nil {
		panic(err)
	}
	fmt.Println(q)
}
