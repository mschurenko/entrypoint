package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"os"
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

func init() {
	r := getRegion()
	sess = session.Must(session.NewSession(&aws.Config{Region: aws.String(r)}))
}

func getRegion() string {
	// use AWS_REGION if set
	if v := os.Getenv("AWS_REGION"); v != "" {
		return v
	}

	client := &http.Client{Timeout: 5 * time.Second}
	r, err := client.Get("http://169.254.169.254/latest/meta-data/placement/availability-zone")
	if err != nil {
		log.Fatalf("could not connect to ECS metadata: %v\n", err)
	}

	bs, err := ioutil.ReadAll(r.Body)

	return string(bs[0 : len(bs)-1])
}

func init() {
	funcMap = map[string]interface{}{
		"getSecret":      getSecret,
		"getNumCPU":      getNumCPU,
		"getNameServers": getNameServers,
		"getHostname":    getHostname,
		"mulf":           func(a, b float64) float64 { return a * b },
	}

	for k, v := range sprig.FuncMap() {
		funcMap[k] = v
	}
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
		log.Fatal(err)
	}

	return *output.SecretString
}

func getNumCPU() float64 {
	f := "/proc/cpuinfo"
	bs, err := ioutil.ReadFile(f)
	if err != nil {
		log.Fatal(err)
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
		log.Fatal(err)
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
		log.Fatal(err)
	}

	return s
}

func getVarsFromS3(s3Path string) map[string]interface{} {
	xs := strings.Split(s3Path, "/")

	var filtered []string
	for i := 0; i < len(xs); i++ {
		if xs[i] != "" {
			filtered = append(filtered, xs[i])
		}
	}

	if len(filtered) < 2 {
		log.Fatalf("%v is not a valid path", s3Path)
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
		log.Fatal(err)
	}

	bs, err := ioutil.ReadAll(o.Body)
	if err != nil {
		log.Fatal(err)
	}

	d := make(map[string]interface{})
	err = yaml.Unmarshal(bs, &d)
	if err != nil {
		log.Fatal(err)
	}

	return d
}

func renderTmpl(tmplFile string, ctx interface{}) {
	newFile := stripExt(tmplFile)

	t := template.Must(template.New(tmplFile).Funcs(funcMap).Option(templateOptions...).ParseFiles(tmplFile))

	f, err := os.Create(newFile)
	if err != nil {
		log.Fatal(err)
	}
	err = t.Execute(f, ctx)
	if err != nil {
		log.Fatal(err)
	}
}

func renderStr(name, tmpl string, ctx interface{}) string {
	t := template.Must(template.New(name).Funcs(funcMap).Option(templateOptions...).Parse(tmpl))

	var b bytes.Buffer
	err := t.Execute(&b, ctx)
	if err != nil {
		log.Fatal(err)
	}

	return b.String()
}
