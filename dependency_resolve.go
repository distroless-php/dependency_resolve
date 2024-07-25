//go:build linux

package main

import (
	"debug/elf"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/u-root/u-root/pkg/ldd"
)

type smartDict struct {
	mu sync.RWMutex
	m  map[string]struct{}
}

func newSmartDict(allocSize int) *smartDict {
	return &smartDict{m: make(map[string]struct{}, allocSize)}
}

func (s *smartDict) appendAll(keys ...string) {
	s.mu.Lock()
	for _, key := range keys {
		s.m[key] = struct{}{}
	}
	s.mu.Unlock()
}

func (s *smartDict) keys() []string {
	s.mu.RLock()
	keys := make([]string, 0, len(s.m))
	for k := range s.m {
		keys = append(keys, k)
	}
	s.mu.RUnlock()
	return keys
}

func version() {
	fmt.Println("3.1.0")
}

func usage() {
	fmt.Println("Usage: dependency_resolve <file1> <file2> ...")
}

func checkBins(bins []string) []string {
	for _, v := range bins {
		if !filepath.IsAbs(v) {
			log.Fatalf("Arguments must be absolute path: %s\n", v)
		}

		// allow symlink
		fi, err := os.Stat(v)
		if err != nil {
			log.Fatalf("os.Stat error: %v\n", err)
		}

		if fi.IsDir() {
			log.Fatalf("Argument is directory: %v\n", v)
		}

		if !(fi.Mode().Perm()&0111 != 0) {
			log.Fatalf("Cannot executable file: %s\n", v)
		}
	}

	return bins
}

func depResolves(sd *smartDict, bins ...string) {
	var wg sync.WaitGroup

	sd.appendAll(bins...)
	for _, bin := range bins {
		wg.Add(1)
		go func(bin string, sd *smartDict, wg *sync.WaitGroup) {
			defer wg.Done()

			fi, err := os.Lstat(bin)
			if err != nil {
				log.Fatalf("os.Lstat failed: %s (%v)\n", bin, err)
			}

			if fi.Mode()&os.ModeSymlink != 0 {
				// symlink
				lp, err := os.Readlink(bin)
				if err != nil {
					log.Fatalf("os.Readlink failed: %s (%v)\n", bin, err)
				}

				if !filepath.IsAbs(bin) {
					lp = filepath.Join(filepath.Dir(bin), lp)
				}

				depResolves(sd, lp)
			} else if _, err := elf.Open(bin); err == nil {
				// bin
				deps, err := ldd.FList(bin)
				if err != nil {
					log.Fatalf("ldd.FList failed: %s (%v)\n", bin, err)
				}

				sd.appendAll(deps...)
			}
		}(bin, sd, &wg)
	}
	wg.Wait()
}

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	switch os.Args[1] {
	case "-h", "--help":
		usage()
		os.Exit(0)
	case "-v", "--version":
		version()
		os.Exit(0)
	}

	bins := checkBins(os.Args[1:])
	sd := newSmartDict(len(bins))
	depResolves(sd, bins...)

	libs := sd.keys()
	slices.Sort(libs)
	for _, v := range libs {
		fmt.Println(v)
	}
}
