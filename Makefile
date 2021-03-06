test:
	docker run --rm -t \
	-v `pwd`:/go/src/entrypoint \
	-e AWS_REGION="us-west-2" \
	-e AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}" \
	-e AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}" \
	-e MY_ENV="testing" \
	-w /go/src/entrypoint \
	golang:latest go test -v -cover

test_container:
	docker run --rm -ti \
	-v `pwd`:/go/src/entrypoint \
	-e AWS_REGION="us-west-2" \
	-e AWS_ACCESS_KEY_ID="${AWS_ACCESS_KEY_ID}" \
	-e AWS_SECRET_ACCESS_KEY="${AWS_SECRET_ACCESS_KEY}" \
	-e ENTRYPOINT_VARS_FILE="/go/src/entrypoint/fixtures/vars-no-secret.yml" \
	-e ENTRYPOINT_TEMPLATES="test1.conf.tmpl" \
	-w /go/src/entrypoint \
	golang:latest ./test.sh

build: test test_container
	docker run --rm -t
	-v `pwd`:/go/src/entrypoint \
	-w /go/src/entrypoint \
	golang:latest go install

PHONY: test test_container build
