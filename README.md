# entrypoint
[![release](http://img.shields.io/github/release/mschurenko/entrypoint.svg?style=flat-square)](https://github.com/mschurenko/entrypoint/releases)
[![CircleCI](https://circleci.com/gh/mschurenko/entrypoint.svg?style=svg)](https://circleci.com/gh/mschurenko/entrypoint)

docker entrypoint that renders go templates


## Template Functions
http://masterminds.github.io/sprig/


## Special Environment Variales
The following environment variables are specfic to `entrypoint` and will not be passed into your container:
```
ENTRYPOINT_TEMPLATES
ENTRYPOINT_TMPL_OPTION
```


## Add this to your Dockerfile(s)
```dockerfile
RUN curl -L https://github.com/mschurenko/entrypoint/releases/download/0.1.11/entrypoint \
  -o /entrypoint && chmod +x /entrypoint

# templates
WORKDIR /conf
COPY my_app.conf.tmpl .

ENTRYPOINT ["/entrypoint"]
```

## Run your docker container
```sh
docker run \
-e ENTRYPOINT_TEMPLATES="/conf/my_app.conf.tmpl" \
my_image:latest \
my_app /conf/my_app.conf
```
