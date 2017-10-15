package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"strconv"

	"github.com/coredns/coredns/core/dnsserver"

	"github.com/mholt/caddy"
)

// Parse all README.md's of the plugin and check if every example Corefile
// actually works. Each corefile is only used if the language is set to 'corefile':
//
// ~~~ corefile
// . {
//	# check-this-please
// }
// ~~~

var dir = flag.String("dir", ".", "directory to crawl")

func main() {
	flag.Parse()
	log.SetOutput(ioutil.Discard)

	files, err := ioutil.ReadDir(*dir)
	if err != nil {
		log.Fatalf("[FATAL] Could not read %s: %q", *dir, err)
	}
	for _, f := range files {
		if !f.IsDir() {
			continue
		}
		if err := checkCorefiles(f.Name()); err != nil {
			log.Printf("[WARNING] %s", err)
		}
	}
}

func checkCorefiles(readme string) error {
	port := 30053
	caddy.Quiet = true
	dnsserver.Quiet = true

	inputs, err := corefileFromFile(readme)
	if err != nil {
		return err
	}

	// Test each snippet.
	for _, in := range inputs {
		dnsserver.Port = strconv.Itoa(port)
		server, err := caddy.Start(in)
		if err != nil {
			fmt.Errorf("Failed to start server with %s, for input %q:\n%s", readme, err, in.Body())
		}
		server.Stop()
		port++
	}
	return nil
}

// corefileFromFile parses a file and returns all fragments that
// have ~~~ corefile (or ``` corefile).
func corefileFromFile(file string) ([]*Input, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	input := []*Input{}
	corefile := false
	temp := ""

	for s.Scan() {
		line := s.Text()
		if line == "~~~ corefile" || line == "``` corefile" {
			corefile = true
			continue
		}

		if corefile && (line == "~~~" || line == "```") {
			// last line
			input = append(input, NewInput(temp))

			temp = ""
			corefile = false
			continue
		}

		if corefile {
			temp += line + "\n" // readd newline stripped by s.Text()
		}
	}

	if err := s.Err(); err != nil {
		return nil, err
	}
	return input, nil
}
