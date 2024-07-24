package main

import (
	"bufio"
	"debug/elf"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"sync"
)

type smartDict struct {
	mu sync.RWMutex
	m  map[string]struct{}
}

func newSmartDict(allocSize int) *smartDict {
	return &smartDict{m: make(map[string]struct{}, allocSize)}
}

func (s *smartDict) exists(key string) bool {
	s.mu.RLock()
	_, exists := s.m[key]
	s.mu.RUnlock()
	return exists
}

func (s *smartDict) append(key string) {
	s.mu.Lock()
	s.m[key] = struct{}{}
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
	fmt.Println("3.0.1")
}

func usage() {
	fmt.Println("Usage: dependency_resolve <file1> <file2> ...")
}

func prepareExec(args []string) {
	if runtime.GOOS != "linux" {
		log.Fatalln("dependency_resolve supports Linux only.")
		os.Exit(1)
	}

	if len(args) < 2 {
		usage()
		os.Exit(2)
	}
}

func checkBins(bins []string) []string {
	for _, v := range bins {
		if !filepath.IsAbs(v) {
			log.Fatalf("Arguments must be absolute path: %s\n", v)
		}

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

func depResolve(wg *sync.WaitGroup, sd *smartDict, ldpaths []string, path string) {
	if sd.exists(path) {
		return
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		fi, err := os.Lstat(path)
		if err != nil {
			log.Fatalf("os.Lstat failed: %s, error: %v", path, err)
		}

		sd.append(path)
		if fi.Mode()&os.ModeSymlink != 0 {
			// symlink
			lp, err := os.Readlink(path)
			if err != nil {
				log.Fatalf("os.Readlink failed: %s, error: %v", path, err)
			}

			if !filepath.IsAbs(lp) {
				lp = filepath.Join(filepath.Dir(path), lp)
			}

			depResolve(wg, sd, ldpaths, lp)
		} else {
			f, err := elf.Open(path)
			if err != nil {
				// Not ELF file
				return
			}
			defer f.Close()

			libs, err := f.ImportedLibraries()
			if err != nil {
				log.Fatalf("elf.ImportedLibraries() failed: %s, error: %v", path, err)
			}

			for _, lib := range libs {
				wg.Add(1)
				go func() {
					defer wg.Done()
					for _, ldpath := range ldpaths {
						fpath := filepath.Join(ldpath, lib)
						if _, err := os.Stat(fpath); err == nil {
							depResolve(wg, sd, ldpaths, fpath)
							break
						}
					}
				}()
			}
		}
	}()
}

func readLdSoConf(file string, paths *[]string) error {
	fp, err := os.Open(file)
	if err != nil {
		return err
	}
	defer fp.Close()

	scanner := bufio.NewScanner(fp)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		if strings.HasPrefix(line, "include ") {
			matches, err := filepath.Glob(strings.TrimSpace(strings.TrimPrefix(line, "include ")))
			if err != nil {
				return err
			}
			for _, match := range matches {
				if err := readLdSoConf(match, paths); err != nil {
					return err
				}
			}
		} else {
			*paths = append(*paths, line)
		}
	}

	return scanner.Err()
}

func getDefaultLdPaths() ([]string, error) {
	var paths []string

	for _, v := range []string{"/etc/ld.so.conf"} {
		if _, err := os.Stat(v); err == nil {
			if err := readLdSoConf(v, &paths); err != nil {
				return nil, err
			}
		}
	}

	for _, v := range []string{"/lib", "/usr/lib"} {
		if d, err := os.Stat(v); err == nil && d.IsDir() {
			paths = append(paths, v)
		}
	}

	return paths, nil
}

func main() {
	prepareExec(os.Args)

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
	ldpaths, err := getDefaultLdPaths()
	if err != nil {
		log.Fatalf("Failed get LDPATH: %v", err)
	}

	var wg sync.WaitGroup
	for _, bin := range bins {
		depResolve(&wg, sd, ldpaths, bin)
	}
	wg.Wait()

	libs := sd.keys()
	slices.Sort(libs)
	for _, v := range libs {
		fmt.Println(v)
	}
}
