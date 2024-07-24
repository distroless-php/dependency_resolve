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
	m  map[string]bool
}

func newSmartDict(allocSize int) *smartDict {
	return &smartDict{m: make(map[string]bool, allocSize)}
}

func (s *smartDict) Exists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.m[key]
	return exists
}

func (s *smartDict) Append(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = true
}

func (s *smartDict) AppendAll(keys []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range keys {
		s.m[v] = true
	}
}

func (s *smartDict) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.m))
	for k := range s.m {
		keys = append(keys, k)
	}
	return keys
}

func (s *smartDict) Remove(key string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
}

func (s *smartDict) RemoveAll(keys []string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, v := range keys {
		delete(s.m, v)
	}
}

func version() {
	fmt.Println("3.0.0")
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

func depResolve(sd *smartDict, ldpaths []string, path string) {
	if sd.Exists(path) {
		return
	}

	var wg sync.WaitGroup
	wg.Add(1)

	go func() {
		defer wg.Done()

		fi, err := os.Lstat(path)
		if err != nil {
			panic("os.Lstat failed: " + path)
		}

		sd.Append(path)
		if fi.Mode()&os.ModeSymlink != 0 {
			// symlink
			lp, err := os.Readlink(path)
			if err != nil {
				panic("os.Readlink failed: " + path)
			}

			if !filepath.IsAbs(lp) {
				lp = filepath.Join(filepath.Dir(path), lp)
			}

			depResolve(sd, ldpaths, lp)
		} else {
			f, err := os.Open(path)
			if err != nil {
				panic("os.Open failed: " + path)
			}

			magic := make([]byte, 4)
			_, err = f.Read(magic)
			if err != nil {
				panic("file.Read failed: " + path)
			}

			if string(magic) == "\x7FELF" {
				// ELF
				elf, err := elf.Open(path)
				if err != nil {
					panic("elf.Open failed: " + path)
				}
				defer elf.Close()

				libs, err := elf.ImportedLibraries()
				if err != nil {
					panic("elf.ImportedLibraries() failed: " + path)
				}

				for _, lib := range libs {
					for _, ldpath := range ldpaths {
						fpath := filepath.Join(ldpath, lib)
						if _, err := os.Stat(fpath); err == nil {
							depResolve(sd, ldpaths, fpath)
							break
						}
					}
				}
			}
		}
	}()

	wg.Wait()
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
	case "-h":
		fallthrough
	case "--help":
		usage()
		os.Exit(0)
	case "-v":
		fallthrough
	case "--version":
		version()
		os.Exit(0)
	}

	bins := checkBins(os.Args[1:])
	sd := newSmartDict(len(bins))
	ldpaths, err := getDefaultLdPaths()
	if err != nil {
		log.Fatalf("Failed get LDPATH: %v", err)
	}

	for _, bin := range bins {
		depResolve(sd, ldpaths, bin)
	}

	libs := sd.Keys()
	slices.Sort(libs)
	for _, v := range libs {
		fmt.Println(v)
	}
}
