package fetch

import (
	"io"
	"net/http"
	"strings"

	"re.vi/pkg"
)

type res struct {
	pkg *pkg.Pkg
	err error
}

func fetchBase(url string, deps []func() (*pkg.Pkg, error), target string) (*pkg.Pkg, error) {
	body, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer body.Body.Close()
	bd, err := io.ReadAll(body.Body)
	if err != nil {
		return nil, err
	}
	pc := make(chan res)
	for _, d := range deps {
		dd := d
		go func() {
			a, b := dd()
			pc <- res{pkg: a, err: b}
		}()
	}
	pkgs := []*pkg.Pkg{}
	for range deps {
		r := <-pc
		if r.err != nil {
			return nil, r.err
		}
		pkgs = append(pkgs, r.pkg)
	}
	return &pkg.Pkg{SourceTarball: bd, Deps: pkgs, Url: url, TargetFile: target}, nil
}
func Fetch(url string) (*pkg.Pkg, error) {
	db, err := http.Get(url + ".deps")
	be := ""
	da := []func() (*pkg.Pkg, error){}
	if err == nil {
		defer db.Body.Close()
		bd, err := io.ReadAll(db.Body)
		if err != nil {
			return nil, err
		}
		be = string(bd)
	}
	s := strings.Split(be, "\n")
	target := ""
	if len(s) == 0 {
		if strings.Contains(url, "gcc") && url != "http://s.minos.io/archive/bifrost/x86_64/gcc-4.6.1-2.tar.gz" {
			s = append(s, "http://s.minos.io/archive/bifrost/x86_64/gcc-4.6.1-2.tar.gz", "http://s.minos.io/archive/morpheus/x86_64/busybox-1.22.1.tar.gz", "http://s.minos.io/archive/morpheus/x86_64/make-3.82.tar.gz")
		}
		if strings.Contains(url, "make") && url != "http://s.minos.io/archive/morpheus/x86_64/make-3.82.tar.gz" {
			s = append(s, "https://mirrorservice.org/sites/sourceware.org/pub/gcc/releases/gcc-11.2.0/gcc-11.2.0.tar.gz")
		}
		if strings.Contains(url, "cmake") {
			s = append(s, "http://mirrors.kernel.org/gnu/make/make-4.3.tar.gz")
		}
		if strings.Contains(url, "ninja") {
			s = append(s, "https://github.com/Kitware/CMake/releases/download/v3.22.0-rc1/cmake-3.22.0-rc1.tar.gz")
		}
		if strings.Contains(url, "python") {
			s = append(s, "http://mirrors.kernel.org/gnu/make/make-4.3.tar.gz")
		}
		if strings.Contains(url, "meson") {
			s = append(s, "https://www.python.org/ftp/python/3.10.0/Python-3.10.0.tgz")
		}
		if strings.Contains(url, "https://github.com/bazelbuild/bazel/releases/download") && !strings.Contains(url, "tar") {
			target = "/bin/bazel"
			s = append(s, "http://s.minos.io/archive/morpheus/x86_64/busybox-1.22.1.tar.gz")
		}
		if strings.Contains(url, "https://github.com/bazelbuild/bazel/releases/download") && strings.Contains(url, "tar") {
			s = append(s, "https://github.com/bazelbuild/bazel/releases/download/4.2.1/bazel-4.2.1-linux-x86_64", "https://www.python.org/ftp/python/3.10.0/Python-3.10.0.tgz")
		}
	}
	for _, f := range s {
		ff := f
		da = append(da, func() (*pkg.Pkg, error) {
			return Fetch(ff)
		})
	}
	return fetchBase(url, da, target)
}
