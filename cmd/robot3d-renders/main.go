// Command robot3d-renders renders the robot pictures listed in renders.json into renders/:
// stills as PNG (transparent unless a bg is set), animations as GIF. Run it from the repository's
// root after changing the model, a screen or the list; the same inputs give the same bytes.
//
//	go run ./cmd/robot3d-renders                 # all of them
//	go run ./cmd/robot3d-renders -only pet,qr    # some
//	go run ./cmd/robot3d-renders -check          # fails if renders/ is not up to date
package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"

	"github.com/i41-eu/i41eu-assets/internal/catalog"
)

func main() {
	list := flag.String("renders", "renders.json", "the list of renders")
	out := flag.String("out", "renders", "where the pictures go")
	only := flag.String("only", "", "comma separated names; empty: all")
	check := flag.Bool("check", false, "write nothing; fail if a picture would change")
	flag.Parse()

	rs, err := catalog.Load(*list)
	if err != nil {
		fail("%v", err)
	}
	want := map[string]bool{}
	for _, n := range strings.Split(*only, ",") {
		if n = strings.TrimSpace(n); n != "" {
			want[n] = true
		}
	}
	base := filepath.Dir(*list)
	if err := os.MkdirAll(*out, 0o755); err != nil {
		fail("%v", err)
	}
	type result struct {
		name, state string
		err         error
	}
	var (
		wg      sync.WaitGroup
		sem     = make(chan struct{}, runtime.NumCPU())
		results = make([]result, len(rs.Renders))
	)
	for i, r := range rs.Renders {
		if len(want) > 0 && !want[r.Name] {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			b, err := r.Make(base)
			if err != nil {
				results[i] = result{r.File(), "", err}
				return
			}
			path := filepath.Join(*out, r.File())
			state := "new"
			if old, err := os.ReadFile(path); err == nil {
				state = "changed"
				if bytes.Equal(old, b) {
					state = "same"
				}
			}
			if !*check && state != "same" {
				err = os.WriteFile(path, b, 0o644)
			}
			results[i] = result{r.File(), state, err}
		}()
	}
	wg.Wait()
	stale := false
	for _, r := range results {
		switch {
		case r.name == "":
		case r.err != nil:
			fail("%s: %v", r.name, r.err)
		default:
			fmt.Printf("%-8s %s\n", r.state, r.name)
			stale = stale || r.state != "same"
		}
	}
	if *check && stale {
		fail("renders/ is not up to date: run robot3d-renders")
	}
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "robot3d-renders: "+format+"\n", a...)
	os.Exit(1)
}
