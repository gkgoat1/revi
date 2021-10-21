package pkg

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
)

type Cfg struct {
	ExtraDeps     []*Pkg
	Patch         map[string][]byte
	CanonicalName []byte
	Chroot        string
}
type Pkg struct {
	SourceTarball []byte
	Deps          []*Pkg
	Url           string
	TargetFile    string
	Cfg           Cfg
}

func (p *Pkg) AllDeps() []*Pkg {
	return append(p.Deps, p.Cfg.ExtraDeps...)
}
func ExtractTarGz(gzipStream io.Reader, prefix string, target string) error {
	if target != "" {
		f, err := os.Create(prefix + target)
		if err != nil {
			return err
		}
		defer f.Close()
		_, err = io.Copy(f, gzipStream)
		return err
	}
	uncompressedStream, err := gzip.NewReader(gzipStream)
	if err != nil {
		return err
	}

	tarReader := tar.NewReader(uncompressedStream)

	for {
		header, err := tarReader.Next()

		if err == io.EOF {
			break
		}

		if err != nil {
			return err
		}

		switch header.Typeflag {
		case tar.TypeDir:
			if err := os.Mkdir(prefix+header.Name, 0755); err != nil {
				return err
			}
		case tar.TypeReg:
			outFile, err := os.Create(prefix + header.Name)
			if err != nil {
				return err
			}
			if _, err := io.Copy(outFile, tarReader); err != nil {
				return err
			}
			outFile.Close()

		default:
		}

	}
	return nil
}
func (p *Pkg) DepFromUrl(u string) *Pkg {
	for _, d := range p.Deps {
		if d.Url == u {
			return d
		}
		if d.DepFromUrl(u) != nil {
			return d.DepFromUrl(u)
		}
	}
	return nil
}
func (p *Pkg) DepFromHash(h string) *Pkg {
	for _, d := range p.Deps {
		if d.Hash() == h {
			return d
		}
		if d.DepFromHash(h) != nil {
			return d.DepFromHash(h)
		}
	}
	return nil
}
func (p *Pkg) Hash() string {
	h := sha256.New()
	h.Write(p.SourceTarball)
	for _, d := range p.AllDeps() {
		h.Write([]byte(d.Hash()))
	}
	for k, p2 := range p.Cfg.Patch {
		h.Write([]byte(k + ":"))
		h.Write(p2)
	}
	h.Write(p.Cfg.CanonicalName)
	h.Write([]byte(p.Cfg.Chroot))
	f, err := os.Open("/proc/self/exe")
	if err != nil {
		panic(err)
	}
	defer f.Close()
	_, err = io.Copy(h, f)
	if err != nil {
		panic(err)
	}
	return fmt.Sprintf("%x", h.Sum(nil))
}
func (p *Pkg) Unpack() error {
	var err error
	uc := make(chan error)
	for _, d := range p.AllDeps() {
		dd := d
		go func() {
			uc <- dd.Unpack()
		}()
	}
	for range p.AllDeps() {
		err = <-uc
		if err != nil {
			return err
		}
	}
	err = ExtractTarGz(bytes.NewReader(p.SourceTarball), p.Path(), p.TargetFile)
	if err != nil {
		return err
	}
	for k, p2 := range p.Cfg.Patch {
		err = os.WriteFile(p.Path()+k, p2, 0755)
		if err != nil {
			return err
		}
	}
	return os.WriteFile(p.Path()+".url", []byte(p.Url), 0755)
}
func (p *Pkg) Run(env map[string]string) error {
	var err error
	for _, d := range p.AllDeps() {
		err = d.Run(env)
		if err != nil {
			return err
		}
	}
	env["LD_LIBRARY_PATH"] = env["LD_LIBRARY_PATH"] + ":" + p.Path() + "lib" + ":" + p.Path() + "usr/lib"
	env["PATH"] = env["PATH"] + ":" + p.Path() + "bin" + ":" + p.Path() + "usr/bin"
	return nil
}

var bmap map[string]bool

func (p *Pkg) Build() error {
	var err error
	for bmap[p.Hash()] {
	}
	bmap[p.Hash()] = true
	defer func() { bmap[p.Hash()] = false }()
	uc := make(chan error)
	paths := []string{}
	for _, d := range p.AllDeps() {
		dd := d
		go func() {
			uc <- dd.Build()
		}()
		paths = append(paths, dd.Path())
	}
	for range p.AllDeps() {
		err = <-uc
		if err != nil {
			return err
		}
	}
	if _, err := os.Stat(p.Path() + "/.re.vi"); err == nil {
		cmd := exec.Command("bwrap", append([]string{"--ro-bind", "/", "/", "--bind", p.Path(), p.Path(), p.Path() + "/.re.vi", p.Path()}, paths...)...)
		cmd.Dir = p.Path()
		return cmd.Run()
	} else if os.IsNotExist(err) {
		if strings.Contains(p.Url, "gcc") && p.Url != "http://s.minos.io/archive/bifrost/x86_64/gcc-4.6.1-2.tar.gz" {
			cmd := exec.Command("bwrap", append([]string{"--ro-bind", "/", "/", "--bind", p.Path(), p.Path(), "sh", "-c", "p=\"$1\";shift 1;cd \"$p\";exec ./configure \"$@\"", "--", p.Path(), "--prefix=" + p.Path()})...)
			cmd.Env = append(cmd.Env, "CC="+p.DepFromUrl("http://s.minos.io/archive/bifrost/x86_64/gcc-4.6.1-2.tar.gz").Path()+"/bin/gcc",
				"CXX="+p.DepFromUrl("http://s.minos.io/archive/bifrost/x86_64/gcc-4.6.1-2.tar.gz").Path()+"/bin/g++")
			err = cmd.Run()
			if err != nil {
				return err
			}
			cmd = exec.Command(p.DepFromUrl("http://s.minos.io/archive/morpheus/x86_64/make-3.82.tar.gz").Path()+"/bin/make", "DESTDIR="+p.Path(), "install")
			cmd.Dir = p.Path()
			return cmd.Run()
		} else if strings.Contains(p.Url, "make") && p.Url != "http://s.minos.io/archive/morpheus/x86_64/make-3.82.tar.gz" {
			cmd := exec.Command("bwrap", append([]string{"--ro-bind", "/", "/", "--bind", p.Path(), p.Path(), "sh", "-c", "p=\"$1\";shift 1;cd \"$p\";exec ./configure \"$@\"", "--", p.Path(), "--prefix=" + p.Path()})...)
			cmd.Env = append(cmd.Env, "CC="+p.DepFromUrl("https://mirrorservice.org/sites/sourceware.org/pub/gcc/releases/gcc-11.2.0/gcc-11.2.0.tar.gz").Path()+"/bin/gcc",
				"CXX="+p.DepFromUrl("https://mirrorservice.org/sites/sourceware.org/pub/gcc/releases/gcc-11.2.0/gcc-11.2.0.tar.gz").Path()+"/bin/g++")
			err = cmd.Run()
			if err != nil {
				return err
			}
			cmd = exec.Command(p.DepFromUrl("http://s.minos.io/archive/morpheus/x86_64/make-3.82.tar.gz").Path()+"/bin/make", "DESTDIR="+p.Path(), "install")
			cmd.Dir = p.Path()
			return cmd.Run()
		} else if strings.Contains(p.Url, "cmake") {
			dp := p.DepFromUrl("http://mirrors.kernel.org/gnu/make/make-4.3.tar.gz").Path()
			cmd := exec.Command("bwrap", append([]string{"--ro-bind", "/", "/", "--bind", p.Path(), p.Path(), "sh", "-c", "p=\"$1\";shift 1;cd \"$p\";exec ./bootstrap \"$@\"", "--", p.Path(), "--", "-DCMAKE_BUILD_TYPE:STRING=Release"})...)
			cmd.Env = append(cmd.Env, "CC="+p.DepFromUrl("https://mirrorservice.org/sites/sourceware.org/pub/gcc/releases/gcc-11.2.0/gcc-11.2.0.tar.gz").Path()+"/bin/gcc",
				"CXX="+p.DepFromUrl("https://mirrorservice.org/sites/sourceware.org/pub/gcc/releases/gcc-11.2.0/gcc-11.2.0.tar.gz").Path()+"/bin/g++")
			err = cmd.Run()
			if err != nil {
				return err
			}
			cmd = exec.Command(dp + "/bin/make")
			cmd.Dir = p.Path()
			return cmd.Run()
		} else if strings.Contains(p.Url, "python") {
			dp := p.DepFromUrl("http://mirrors.kernel.org/gnu/make/make-4.3.tar.gz").Path()
			cmd := exec.Command("bwrap", append([]string{"--ro-bind", "/", "/", "--bind", p.Path(), p.Path(), "sh", "-c", "p=\"$1\";shift 1;cd \"$p\";exec ./configure \"$@\"", "--", p.Path(), "--prefix=" + p.Path()})...)
			cmd.Env = append(cmd.Env, "CC="+p.DepFromUrl("https://mirrorservice.org/sites/sourceware.org/pub/gcc/releases/gcc-11.2.0/gcc-11.2.0.tar.gz").Path()+"/bin/gcc",
				"CXX="+p.DepFromUrl("https://mirrorservice.org/sites/sourceware.org/pub/gcc/releases/gcc-11.2.0/gcc-11.2.0.tar.gz").Path()+"/bin/g++")
			err = cmd.Run()
			if err != nil {
				return err
			}
			cmd = exec.Command(dp + "/bin/make")
			cmd.Dir = p.Path()
			return cmd.Run()
		} else if strings.Contains(p.Url, "ninja") {
			dp := p.DepFromUrl("http://mirrors.kernel.org/gnu/make/make-4.3.tar.gz").Path()
			cmd := exec.Command("bwrap", append([]string{"--ro-bind", "/", "/", "--bind", p.Path(), p.Path(), "sh", "-c", "p=\"$1\";shift 1;exec cmake -S \"$p\" -Bbin", "--", p.Path(), "-DCMAKE_BUILD_TYPE:STRING=Release"})...)
			cmd.Env = append(cmd.Env, "CC="+p.DepFromUrl("https://mirrorservice.org/sites/sourceware.org/pub/gcc/releases/gcc-11.2.0/gcc-11.2.0.tar.gz").Path()+"/bin/gcc",
				"CXX="+p.DepFromUrl("https://mirrorservice.org/sites/sourceware.org/pub/gcc/releases/gcc-11.2.0/gcc-11.2.0.tar.gz").Path()+"/bin/g++")
			err = cmd.Run()
			if err != nil {
				return err
			}
			cmd = exec.Command(dp+"/bin/make", []string{"-C", "bin"}...)
			cmd.Dir = p.Path()
			return cmd.Run()
		} else if strings.Contains(p.Url, "meson") {
			dp := p.DepFromUrl("https://www.python.org/ftp/python/3.10.0/Python-3.10.0.tgz").Path()
			cmd := exec.Command(dp+"/bin/python", "-m", "venv", p.Path())
			err = cmd.Run()
			if err != nil {
				return err
			}
			cmd = exec.Command(dp+"/bin/python", "-m", "pip", "install", "meson")
			cmd.Env = append(cmd.Env, "VIRTUAL_ENV="+p.Path())
			err = cmd.Run()
			if err != nil {
				return err
			}
			return nil
		} else if strings.Contains(p.Url, "https://github.com/bazelbuild/bazel/releases/download") && strings.Contains(p.Url, "tar") {
			bd := p.DepFromUrl("https://github.com/bazelbuild/bazel/releases/download/4.2.1/bazel-4.2.1-linux-x86_64").Path()
			cmd := exec.Command(bd+"/bin/bazel", "build", "//src:bazel-dev")
			err = cmd.Run()
			if err != nil {
				return err
			}
			return exec.Command("/bin/cp", p.Path()+"/bazel-bin/src/bazel-dev", p.Path()+"/bin/bazel").Run()
		} else if _, err := os.Stat(p.Path() + "/bin/busybox"); err == nil {
			cmd := exec.Command(p.Path() + "/bin/busybox")
			o, err := cmd.Output()
			if err != nil {
				return err
			}
			os_ := strings.Split(string(o), ",")
			for _, oo := range os_ {
				a := "/bin/" + oo
				_ = os.Remove(a)
				err = os.Symlink(p.Path()+"/bin/busybox", a)
				if err != nil {
					return err
				}
			}
		} else if os.IsNotExist(err) {

		} else {
			return err
		}
		return nil
	} else {
		return err
	}
}
func (p *Pkg) Path() string {
	return fmt.Sprintf("%s/re/vi/%s/", p.Cfg.Chroot, p.Hash())
}
