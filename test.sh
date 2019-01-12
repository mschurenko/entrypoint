#!/bin/bash

set -eux

go install -race

cd fixtures

entrypoint cat test1.conf test2.conf