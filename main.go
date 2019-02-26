package main

import (
	"fmt"
	"log"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"sync"
	"syscall"
)

// will be set va -ldflags
var version = ""

func main() {
	usage := fmt.Sprintf("Usage: %v cmd [argN...]", os.Args[0])

	if len(os.Args) < 2 {
		log.Fatal(usage)
	}

	execArgs := os.Args[1:]
	log.Println("entrypoint version:", version)
	log.Println("entrypoint arguments:", strings.Join(execArgs, " "))

	cmdPath, err := exec.LookPath(execArgs[0])
	if err != nil {
		log.Fatal(err)
	}

	containerVars := make(map[string]string)
	var templates []string

	// parse ENV vars
	for _, i := range os.Environ() {
		xs := strings.Split(i, "=")
		k := xs[0]
		v := xs[1]

		if strings.HasPrefix(k, "ENTRYPOINT_") {
			if !checkEntrypointVar(k) {
				log.Fatalf("Error: %v is not one of %v", k, entrypointEnvVars)
			}
		} else {
			containerVars[k] = v
		}

		// render any secrets in env vars
		if matched, _ := regexp.Match(`^{{.*}}$`, []byte(v)); matched {
			rv := newTpl(k).renderStr(v)
			// override env var with secret value
			os.Setenv(k, rv)
			containerVars[k] = rv
		}

		if k == "ENTRYPOINT_TEMPLATES" {
			templates = strings.Split(v, ",")
		}
	}

	if len(templates) > 0 {
		wg := sync.WaitGroup{}

		for _, t := range templates {
			wg.Add(1)
			go func(t string) {
				newTpl(t).renderFile()
				wg.Done()
			}(t)
		}

		wg.Wait()

	}

	var containerVarsXs []string
	for k, v := range containerVars {
		containerVarsXs = append(containerVarsXs, k+"="+v)
	}

	err = syscall.Exec(cmdPath, execArgs, containerVarsXs)
	if err != nil {
		log.Fatal(err)
	}
}
