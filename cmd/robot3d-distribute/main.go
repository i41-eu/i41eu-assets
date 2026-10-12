// Command robot3d-distribute copies the rendered robot pictures (renders/) into the repositories
// listed in distribute.json, in each target's format and size. It only writes files: review,
// commit and push in each repository as that repository's rules say.
//
//	go run ./cmd/robot3d-distribute -check   # what is out of date, nothing written (exit 1 if any)
//	go run ./cmd/robot3d-distribute          # write the ones that differ
package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/i41-eu/i41eu-assets/internal/catalog"
)

func main() {
	list := flag.String("renders", "renders.json", "the list of renders")
	dir := flag.String("dir", "renders", "the rendered pictures")
	dist := flag.String("distribute", "distribute.json", "where they go")
	check := flag.Bool("check", false, "write nothing; exit 1 if a target differs")
	flag.Parse()

	rs, err := catalog.Load(*list)
	if err != nil {
		fail("%v", err)
	}
	d, err := catalog.LoadDistribution(*dist)
	if err != nil {
		fail("%v", err)
	}
	res, err := d.Run(*dir, rs, *check)
	for _, r := range res {
		fmt.Printf("%-8s %s:%s  (%s)\n", r.State, r.Repo, r.Path, r.Render)
	}
	if err != nil {
		fail("%v", err)
	}
	changed := map[string]bool{}
	for _, r := range res {
		if r.State != "same" {
			changed[r.Repo] = true
		}
	}
	if len(changed) == 0 {
		fmt.Println("every target is up to date")
		return
	}
	if *check {
		fmt.Printf("%d repositories out of date\n", len(changed))
		os.Exit(1)
	}
	fmt.Println("written; commit them in:")
	for repo := range changed {
		fmt.Println("  " + repo)
	}
}

func fail(format string, a ...any) {
	fmt.Fprintf(os.Stderr, "robot3d-distribute: "+format+"\n", a...)
	os.Exit(1)
}
