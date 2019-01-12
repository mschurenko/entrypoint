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

func main() {
	usage := fmt.Sprintf("Usage: %v cmd [argN...]", os.Args[0])

	fmt.Println(os.Args)

	if len(os.Args) < 2 {
		log.Fatal(usage)
	}

	execArgs := os.Args[1:]

	cmdPath, err := exec.LookPath(execArgs[0])
	if err != nil {
		log.Fatal(err)
	}

	envVars := make(map[string]string)
	for _, i := range os.Environ() {
		s := strings.Split(i, "=")
		envVars[s[0]] = s[1]
	}

	// check for required env vars
	var numMatched int
	for i := range envVars {
		for _, j := range requiredEnvVars {
			if i == j {
				numMatched++
			}
		}
	}

	if numMatched != len(requiredEnvVars) {
		log.Fatalf("required Env vars: %v are not set", requiredEnvVars)
	}

	vars := getVarsFromS3(os.Getenv("ENTRYPOINT_S3_PATH"))

	var templates []string

	// parse ENV vars
	for k, v := range envVars {
		matched, _ := regexp.Match(`^{{.*}}$`, []byte(v))
		if matched {
			// tmplCtx.EnvVars is empty in this context
			rendered := renderStr(k, v, tmplCtx{Vars: vars})
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
				renderTmpl(t, ctx)
				wg.Done()
			}(t)
		}

		wg.Wait()

	}

	fmt.Println(execArgs)

	err = syscall.Exec(cmdPath, execArgs, os.Environ())
	if err != nil {
		log.Fatal(err)
	}
}
