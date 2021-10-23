package main

import (
	"os"
	"os/exec"
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
	} else if os.Args[1] == "ostree-up"{
		var p *pkg.Pkg
		p, err := fetch.Fetch("https://github.com/ostreedev/ostree/releases/download/v2021.5/libostree-2021.5.tar.xz")
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
		td, err := os.MkdirTemp("/tmp/revi","*")
		if err != nil {
			panic(err)
		}
		cmd := exec.Command("bwrap",append([]string{"--bind","/","/","--bind",td+"/re/vi","/re/vi",os.Args[0],"up"},os.Args[3:]...)...)
		err = cmd.Run()
		if err != nil{
			panic(err)
		}
		cmd = exec.Command(p.Path()+"/bin/ostree","--repo",os.Args[2],"commit",td)
		err = cmd.Run()
		if err != nil{
			panic(err)
		}
	}
}
