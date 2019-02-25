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

type tmplCtx struct {
	envVars map[string]string
	vars    map[string]interface{}
}

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

	envVars := make(map[string]string)
	for _, i := range os.Environ() {
		s := strings.Split(i, "=")
		envVars[s[0]] = s[1]
	}

	vars := make(map[string]interface{})
	if os.Getenv("ENTRYPOINT_VARS_FILE") != "" {
		vars = getVarsFromFile(os.Getenv("ENTRYPOINT_VARS_FILE"))
	}

	containerVars := make(map[string]string)
	var templates []string

	// parse ENV vars
	for k, v := range envVars {
		if strings.HasPrefix(k, "ENTRYPOINT_") {
			if !checkEntrypointVar(k) {
				log.Fatalf("Error: %v is not one of %v", k, entrypointEnvVars)
			}
		} else {
			containerVars[k] = v
		}

		if matched, _ := regexp.Match(`^{{.*}}$`, []byte(v)); matched {
			var rendered string
			if len(vars) > 0 {
				rendered = newTpl(k, tmplCtx{vars: vars}).renderStr(v)
			} else {
				rendered = newTpl(k, nil).renderStr(v)
			}
			containerVars[k] = rendered
		}

		if k == "ENTRYPOINT_TEMPLATES" {
			templates = strings.Split(v, ",")
		}
	}

	if len(templates) > 0 {
		ctx := tmplCtx{
			envVars: envVars,
			vars:    vars,
		}

		wg := sync.WaitGroup{}

		for _, t := range templates {
			wg.Add(1)
			go func(t string) {
				newTpl(t, ctx).renderFile()
				wg.Done()
			}(t)
		}

		wg.Wait()

	}

	var containerVarsSlice []string
	for k, v := range containerVars {
		containerVarsSlice = append(containerVarsSlice, k+"="+v)
	}

	err = syscall.Exec(cmdPath, execArgs, containerVarsSlice)
	if err != nil {
		log.Fatal(err)
	}
}
