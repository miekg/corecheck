package main

import (
	"bufio"
	"flag"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"

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

var dir = flag.String("dir", ".", "directory to scan for .md files")

func main() {
	flag.Parse()

	files, err := ioutil.ReadDir(*dir)
	if err != nil {
		log.Fatalf("[FATAL] Could not read %s: %q", *dir, err)
	}
	for _, f := range files {
		if f.IsDir() {
			continue
		}

		fullname := filepath.Join(*dir, f.Name())

		if filepath.Ext(fullname) != ".md" {
			continue
		}
		if err := checkCorefiles(fullname); err != nil {
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
	if len(inputs) == 0 {
		return nil
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
	log.Printf("[INFO] Checking %d snippets in %s: OK", len(inputs), readme)

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
		line = strings.TrimSpace(line)
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
