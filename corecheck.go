package main

import (
	"bufio"
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

// Parse all README.md's of the plugin and check if every example Corefile
// actually works. Each corefile is only used if the language is set to 'corefile':
//
// ~~~ corefile
// . {
//	# check-this-please
// }
// ~~~

var (
	dir = flag.String("dir", ".", "directory to scan for .md files")
	exe = flag.String("exe", "./coredns", "path to coredns executable")
)

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
		checkCorefiles(fullname)
	}
}

func checkCorefiles(readme string) error {
	inputs, err := corefileFromFile(readme)
	if err != nil {
		return err
	}
	if len(inputs) == 0 {
		return nil
	}

	// Test each snippet.
	fail := 0
	fmt.Printf("Checking %d snippets in %s\n", len(inputs), readme)
	for _, in := range inputs {
		buf := make([]byte, 2048)

		server, out, err := coreStart(*exe, in)
		n, _ := out.Read(buf)
		buf = buf[:n]

		if err != nil {
			fmt.Printf("Failed to start server with %s, for input %q:\n%s\n", readme, err, in)
			fail++
			server.Process.Kill()
			continue
		}

		go func() {
			err := server.Wait()
			if err != nil {
				// yech, but so be it
				if strings.Contains(err.Error(), "signal: killed") {
					// OK, killed below.
					return
				}
				if strings.Contains(string(buf), "KUBERNETES_SERVICE_HOST and KUBERNETES_SERVICE_PORT must be defined") {
					// OK, need to be running in k8s cluster
					return
				}
				fmt.Printf("Failed to start server with %s, for input %q: standand error %q\n%s\n", readme, err, string(buf), in)
				fail++
			}
		}()
		time.Sleep(500 * time.Millisecond)
		server.Process.Kill()
	}
	if fail > 0 {
		fmt.Printf("\tFAIL: %d snippets in %s: %d failed\n", len(inputs), readme, fail)
	} else {
		fmt.Printf("\tPASS: %d snippets in %s\n", len(inputs), readme)
	}

	return nil
}

// corefileFromFile parses a file and returns all fragments that
// have ~~~ corefile (or ``` corefile).
func corefileFromFile(file string) ([]string, error) {
	f, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	s := bufio.NewScanner(f)
	input := []string{}
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
			input = append(input, temp)

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

const conffile = "/tmp/corefile-readme"

func coreStart(path, conf string) (*exec.Cmd, io.ReadCloser, error) {
	err := ioutil.WriteFile(conffile, []byte(conf), 0640)
	if err != nil {
		return nil, nil, err
	}

	cmd := exec.Command(path, "-conf", conffile, "-dns.port", "0")

	out, _ := cmd.StderrPipe()

	return cmd, out, cmd.Start()
}
