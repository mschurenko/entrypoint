package main

import (
	"bytes"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"
	"time"

	"github.com/Masterminds/sprig"
	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/secretsmanager"
)

var sess *session.Session
var entrypointEnvVars = []string{
	"ENTRYPOINT_VARS_FILE",
	"ENTRYPOINT_TEMPLATES",
	"ENTRYPOINT_TMPL_OPTION",
}

const tmplExt string = ".tmpl"
const s3Prefix string = "s3://"

func init() {
	r := ec2Metadata("region")
	sess = session.Must(session.NewSession(&aws.Config{Region: aws.String(r)}))
}

func checkEntrypointVar(v string) bool {
	for _, e := range entrypointEnvVars {
		if v == e {
			return true
		}
	}

	return false
}

/*
most of the same arguments as:
https://aws.amazon.com/code/ec2-instance-metadata-query-tool/
*/
func ec2Metadata(path string) string {
	baseURL := "http://169.254.169.254/latest/"
	client := &http.Client{Timeout: 3 * time.Second}

	var r *http.Response
	var err error

	switch path {
	case "ami-id":
		r, err = client.Get(baseURL + "/meta-data/ami-id/")
	case "user-data":
		r, err = client.Get(baseURL + "user-data/")
	case "instance-id":
		r, err = client.Get(baseURL + "/meta-data/instance-id/")
	case "instance-type":
		r, err = client.Get(baseURL + "/meta-data/instance-type/")
	case "ami-launch-index":
		r, err = client.Get(baseURL + "/meta-data/ami-launch-index/")
	case "availability-zone":
		r, err = client.Get(baseURL + "/meta-data/placement/availability-zone/")
	case "region":
		if v := os.Getenv("AWS_REGION"); v != "" {
			return v
		}
		r, err = client.Get(baseURL + "/meta-data/placement/availability-zone/")
	default:
		log.Fatalf("ec2Metadata: unsupported path %s", path)
	}

	if err != nil {
		log.Fatalf("ec2Metadata: %v\n", err)
	}

	bs, err := ioutil.ReadAll(r.Body)
	if path == "region" {
		return string(bs[:len(bs)-1])
	}
	return string(bs)
}

func secret(name string) string {
	svc := secretsmanager.New(sess)
	input := &secretsmanager.GetSecretValueInput{
		SecretId: aws.String(name),
	}

	output, err := svc.GetSecretValue(input)
	if err != nil {
		log.Fatalf("secret: %v", err)
	}

	return *output.SecretString
}

func nameServers() []string {
	bs, err := ioutil.ReadFile("/etc/resolv.conf")
	if err != nil {
		log.Fatalf("nameServers: %v", err)
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

func hostname() string {
	s, err := os.Hostname()
	if err != nil {
		log.Fatalf("hostname: %v", err)
	}

	return s
}

type tpl struct {
	name    string
	output  string
	ctx     interface{}
	opts    []string
	funcMap map[string]interface{}
}

func newTpl(name string) tpl {
	opts := []string{}
	opt := os.Getenv("ENTRYPOINT_TMPL_OPTION")
	switch opt {
	case "default", "invalid", "zero", "error":
		opts = []string{"missingkey=" + opt}
	case "":
		opts = []string{"missingkey=error"}
	default:
		log.Fatalf("%v is not a valid option for text/template", opt)
	}

	funcMap := map[string]interface{}{
		"secret":      secret,
		"numCpu":      runtime.NumCPU,
		"nameServers": nameServers,
		"hostname":    hostname,
		"ec2Metadata": ec2Metadata,
	}
	for k, v := range sprig.FuncMap() {
		funcMap[k] = v
	}

	var output string
	if _, err := os.Stat(name); err == nil {
		output = strings.Replace(strings.Replace(name, ".tpl", "", 1), ".tmpl", "", 1)
	}

	return tpl{
		name:    name,
		output:  output,
		opts:    opts,
		funcMap: funcMap,
	}
}

func (tpl tpl) renderFile() {
	t := template.Must(template.New(filepath.Base(tpl.name)).Funcs(tpl.funcMap).Option(tpl.opts...).ParseFiles(tpl.name))
	f, err := os.Create(tpl.output)
	if err != nil {
		log.Fatalf("renderTmpl: %v", err)
	}
	err = t.Execute(f, nil)
	if err != nil {
		log.Fatalf("renderTmpl: %v", err)
	}
}

func (tpl tpl) renderStr(s string) string {
	t := template.Must(template.New(tpl.name).Funcs(tpl.funcMap).Option(tpl.opts...).Parse(s))

	var b bytes.Buffer
	err := t.Execute(&b, nil)
	if err != nil {
		log.Fatalf("renderStr: %v", err)
	}

	return b.String()
}
