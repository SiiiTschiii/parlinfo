// build_site writes the parlinfo.ch site into a directory.
//
//	go run ./cmd/build_site -out out/site
//
// The site workflow runs it and hands the result to site/publish.sh.
package main

import (
	"flag"
	"log"

	"github.com/siiitschiii/parlinfo/site"
)

func main() {
	out := flag.String("out", "out/site", "directory to write the site into")
	flag.Parse()
	if err := site.Build(*out); err != nil {
		log.Fatal(err)
	}
	log.Printf("Site written to %s", *out)
}
