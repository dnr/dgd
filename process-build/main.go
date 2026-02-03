package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	script := flag.String("script", "", "path to go build -n output")
	rootdir := flag.String("rootdir", "", "root directory of build")
	deproot := flag.String("deproot", "", "root of mod cache (dependencies)")
	flag.Parse()

	if *script == "" || *rootdir == "" {
		log.Fatalln("Usage: process-build -script <file> -rootdir <rootdir> [-deproot <depRoot>]")
	}

	absify(rootdir)
	absify(deproot)

	f, err := os.Open(*script)
	if err != nil {
		log.Fatalln("open:", err)
		os.Exit(1)
	}
	defer f.Close()

	// TODO: get known go package in here
	actions, gopkg, err := Parse(f, "")
	if err != nil {
		log.Fatalln("parse error:", err)
	}

	g, err := Build(actions, gopkg, *rootdir, *deproot)
	if err != nil {
		log.Fatalln("build:", err)
	}

	nixExpr := Generate(g, *rootdir)
	fmt.Println(strings.TrimSpace(nixExpr))
}

func absify(p *string) {
	if *p == "" {
		return
	} else if abs, err := filepath.Abs(*p); err != nil {
		log.Fatalf("error finding absolute path %q: %v", *p, err)
	} else {
		*p = abs
	}
}
