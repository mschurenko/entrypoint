# entrypoint
[![release](http://img.shields.io/github/release/mschurenko/entrypoint.svg?style=flat-square)](https://github.com/mschurenko/entrypoint/releases)
[![CircleCI](https://circleci.com/gh/mschurenko/entrypoint.svg?style=svg)](https://circleci.com/gh/mschurenko/entrypoint)

docker entrypoint that renders go templates

# Usage
## This struct gets passed into templates
```go
type tmplCtx struct {
	EnvVars map[string]string
	Vars    map[string]interface{}
}
```

Which can be referenced like:
```gohtml
value of MY_VAR is {{ .EnvVars["MY_VAR"] }}
```

## Templating files

## Templating Environment Variables
Note that `EnvVars` will not be passed into a template context when using `renderStr`


## Template Functions
```go
funcMap = map[string]interface{}{
		"getSecret":      getSecret,
		"getNumCPU":      getNumCPU,
		"getNameServers": getNameServers,
		"getHostname":    getHostname,
		"getRegion":      getRegion,
		"mulf":           func(a, b float64) float64 { return a * b },
	}
```

In addition to the above github.com/Masterminds/sprig are included.

## Dealing with empty values
By default `entrypoint` sets `template.Option` to `[]string{"missing=error"}`. This can be changed by setting `ENTRYPOINT_TMPL_OPTION`.


## Special Environment Variales
The following environment variables are specfic to `entrypoint` and will not be passed into your container:
```
ENTRYPOINT_VARS
ENTRYPOINT_TEMPLATES
ENTRYPOINT_TMPL_OPTION
```

## Testing templates
