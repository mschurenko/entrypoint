package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
	yaml "gopkg.in/yaml.v2"
)

var sess *session.Session
var funcMap map[string]interface{}
var templateOptions = []string{}
var entrypointEnvVars = []string{
	"ENTRYPOINT_VARS_FILE",
	"ENTRYPOINT_TEMPLATES",
	"ENTRYPOINT_TMPL_OPTION",
}

const tmplExt string = ".tmpl"
const s3Prefix string = "s3://"

func init() {
	r := getRegion()
	sess = session.Must(session.NewSession(&aws.Config{Region: aws.String(r)}))
}

func init() {
	templateOption := os.Getenv("ENTRYPOINT_TMPL_OPTION")
	switch templateOption {
	case "default", "invalid", "zero", "error":
		templateOptions = []string{"missingkey=" + templateOption}
	case "":
		templateOptions = []string{"missingkey=error"}
	default:
		log.Fatalf("%v is not a valid option for text/template", templateOption)
	}
}

func init() {
	funcMap = map[string]interface{}{
		"getSecret":      getSecret,
		"getNumCPU":      getNumCPU,
		"getNameServers": getNameServers,
		"getHostname":    getHostname,
		"getRegion":      getRegion,
		"mulf":           func(a, b float64) float64 { return a * b },
	}

	for k, v := range sprig.FuncMap() {
		funcMap[k] = v
	}
}

func checkEntrypointVar(v string) bool {
	for _, e := range entrypointEnvVars {
		if v == e {
			return true
		}
	}

	return false
}

func getRegion() string {
	// use AWS_REGION if set
	if v := os.Getenv("AWS_REGION"); v != "" {
		return v
	}

	client := &http.Client{Timeout: 5 * time.Second}
	r, err := client.Get("http://169.254.169.254/latest/meta-data/placement/availability-zone")
	if err != nil {
		log.Fatalf("getRegion: could not connect to ECS metadata: %v\n", err)
	}

	bs, err := ioutil.ReadAll(r.Body)

	return string(bs[:len(bs)-1])
}

func stripExt(f string) string {
	return strings.Replace(f, tmplExt, "", 1)
}

func getSecret(name string) string {
	svc := secretsmanager.New(sess)
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	}

	output, err := svc.GetSecretValue(input)
	if err != nil {
		log.Fatalf("getSecret: %v", err)
	}

	return *output.SecretString
}

func getNumCPU() float64 {
	bs, err := ioutil.ReadFile("/proc/cpuinfo")
	if err != nil {
		log.Fatalf("getNumCPU: %v", err)
	}

	lines := strings.Split(string(bs), "\n")

	var numProcs float64
	for _, l := range lines {
		if l == "" {
			continue
		}
		xs := strings.Split(l, ":")
		key := strings.TrimSpace(xs[0])

		if key == "processor" {
			numProcs++
		}
	}

	return numProcs
}

func getNameServers() []string {
	bs, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		log.Fatalf("getNameServers: %v", err)
	}

	var ns []string

	lines := strings.Split(string(bs), "\n")

	for _, l := range lines {
		if strings.HasPrefix(l, "nameserver") {
			ns = append(ns, strings.Fields(l)[1])
		}
	}

	return ns
}

func getHostname() string {
	s, err := os.Hostname()
	if err != nil {
		log.Fatalf("getHostname: %v", err)
	}

	return s
}

func getVarsFromFile(file string) map[string]interface{} {
	var s string

	if strings.HasPrefix(file, s3Prefix) {
		s = getFileFromS3(file)
	} else {
		// assume this is a local file
		bs, err := ioutil.ReadFile(file)
		if err != nil {
			log.Fatalf("getVarsFromFile: %v", err)
		}

		s = string(bs)
	}

	// context is nil for vars file
	y := renderStr("vars", s, nil)

	v := make(map[string]interface{})
	err := yaml.Unmarshal([]byte(y), &v)
	if err != nil {
		log.Fatalf("getVarsFromFile: %v", err)
	}

	return v
}

func getFileFromS3(file string) string {
	s3Path := strings.Split(file, s3Prefix)[1]

	xs := strings.Split(s3Path, "/")

	var filtered []string
	for i := 0; i < len(xs); i++ {
		if xs[i] != "" {
			filtered = append(filtered, xs[i])
		}
	}

	if len(filtered) < 2 {
		log.Fatalf("getFileFromS3: %v is not a valid path", s3Path)
	}
	bucket := filtered[0]
	path := strings.Join(filtered[1:], "/")

	svc := s3.New(sess)

	input := &s3.GetObjectInput{
		Bucket: aws.String(bucket),
		Key:    aws.String(path),
	}

	o, err := svc.GetObject(input)
	if err != nil {
		log.Fatalf("getFileFromS3: %v", err)
	}

	bs, err := ioutil.ReadAll(o.Body)
	if err != nil {
		log.Fatalf("getFileFromS3: %v", err)
	}
	return string(bs)
}

func renderTmpl(tmplFile string, ctx interface{}) {
	newFile := stripExt(tmplFile)

	t := template.Must(template.New(filepath.Base(tmplFile)).Funcs(funcMap).Option(templateOptions...).ParseFiles(tmplFile))

	f, err := os.Create(newFile)
	if err != nil {
		log.Fatalf("renderTmpl: %v", err)
	}
	err = t.Execute(f, ctx)
	if err != nil {
		log.Fatalf("renderTmpl: %v", err)
	}
}

func renderStr(name, tmpl string, ctx interface{}) string {
	t := template.Must(template.New(name).Funcs(funcMap).Option(templateOptions...).Parse(tmpl))

	var b bytes.Buffer
	err := t.Execute(&b, ctx)
	if err != nil {
		log.Fatalf("renderStr: %v", err)
	}

	return b.String()
}
