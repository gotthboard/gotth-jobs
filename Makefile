.PHONY: verify verify-integration

verify:
	version="$$(go env GOVERSION)"; test "$${version%%-*}" = "go1.26.6"
	test -z "$$(gofmt -l $$(git ls-files --cached --others --exclude-standard -- '*.go'))"
	go vet -mod=readonly ./...
	go test -mod=readonly -race -cover ./...

verify-integration:
	test -n "$${GOTTH_JOBS_TEST_DATABASE_URL}"
	test "$${GOTTH_JOBS_ALLOW_DESTRUCTIVE_TEST_DATABASE_RESET}" = "true"
	go test -mod=readonly -race -tags=integration ./...
