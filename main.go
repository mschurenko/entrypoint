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

type tmplCtx struct {
	EnvVars map[string]string
	Vars    map[string]interface{}
}

func main() {
	usage := fmt.Sprintf("Usage: %v cmd [argN...]", os.Args[0])

	if len(os.Args) < 2 {
		log.Fatal(usage)
	}

	execArgs := os.Args[1:]
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

	var containerVars []string
	var templates []string

	// parse ENV vars
	for k, v := range envVars {

		if strings.HasPrefix(k, "ENTRYPOINT_") {
			if !checkEntrypointVar(k) {
				log.Fatalf("Error: %v is not one of %v", k, entrypointEnvVars)
			}
		} else {
			containerVars = append(containerVars, k+"="+v)
		}

		matched, _ := regexp.Match(`^{{.*}}$`, []byte(v))
		if matched {
			var rendered string
			if len(vars) > 0 {
				rendered = newTpl(k, tmplCtx{Vars: vars}).renderStr(v)
			} else {
				rendered = newTpl(k, nil).renderStr(v)
			}
			os.Setenv(k, rendered)
		}
		if k == "ENTRYPOINT_TEMPLATES" {
			templates = strings.Split(v, ",")
		}
	}

	if len(templates) > 0 {
		ctx := tmplCtx{
			EnvVars: envVars,
			Vars:    vars,
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

	err = syscall.Exec(cmdPath, execArgs, containerVars)
	if err != nil {
		log.Fatal(err)
	}
}
