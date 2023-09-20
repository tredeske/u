#!/bin/sh -eux

export GOEXPERIMENT=loopvar # TODO: remove when go 1.22
go build
go test ./...
go vet ./...
exit 0
