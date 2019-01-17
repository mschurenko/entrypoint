package main

const tmplExt string = ".tmpl"

var templateOptions = []string{"missingkey=error"}

type tmplCtx struct {
	EnvVars map[string]string
	Vars    map[string]interface{}
}
