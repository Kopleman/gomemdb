.PHONY: test
test:
	$(info #Not implemented)

.PHONY: _test-coverage
_test-coverage:
	go test -v -coverpkg=./... -coverprofile=profile.cov.tmp ./...

.PHONY: _test-coverage_clean_up
_test-coverage_clean_up:
	cat profile.cov.tmp | grep -v "_gen.go" | grep -v "mock.go" | grep -v "mocks" > cover.out

.PHONY: _coverage_report
_coverage_report:
	go tool cover -func cover.out

.PHONY: coverage
coverage: _test-coverage _test-coverage_clean_up _coverage_report