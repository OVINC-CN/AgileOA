GOLANGCI_LINT_VERSION := v2.15.0
GOLANGCI_LINT ?= go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@$(GOLANGCI_LINT_VERSION)
SWAG_VERSION := v1.16.6
SWAG ?= go run github.com/swaggo/swag/cmd/swag@$(SWAG_VERSION)

.PHONY: dev worker beat build tidy lint format format-check test check swagger swagger-check

dev:
	go run . api

worker:
	go run . worker

beat:
	go run . beat

build:
	go build -trimpath -o bin/agile-workspace .

tidy:
	go mod tidy
	go mod verify

lint:
	$(GOLANGCI_LINT) run

format:
	$(GOLANGCI_LINT) fmt

format-check:
	@test -z "$$(gofmt -l .)" || (gofmt -l .; exit 1)

test:
	go test -race ./...

swagger:
	$(SWAG) init --generalInfo main.go --parseInternal --output internal/httpd/docs --outputTypes json,yaml

swagger-check:
	go run ./scripts/check-swagger

check: swagger swagger-check format-check lint test build
