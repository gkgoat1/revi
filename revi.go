package main

import (
	"os"

	"re.vi/fetch"
	"re.vi/pkg"
)

func main() {
	if os.Args[1] == "up" {
		var p *pkg.Pkg
		p, err := fetch.Fetch(os.Args[2])
		if err != nil {
			panic(err)
		}
		err = p.Unpack()
		if err != nil {
			panic(err)
		}
		err = p.Build()
		if err != nil {
			panic(err)
		}
	}
}
