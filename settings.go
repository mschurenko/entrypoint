package main

const tmplExt string = ".tmpl"

var templateOptions = []string{"missingkey=error"}

var requiredEnvVars = []string{"ENTRYPOINT_S3_PATH"}

type tmplCtx struct {
	EnvVars map[string]string
	Vars    map[string]interface{}
}
