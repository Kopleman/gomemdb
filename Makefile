include scripts/*.mk
CLI_BIN=./bin/cli
CLI_ENTRY=./cmd/cli/main.go
SERVER_BIN=./bin/server
SERVER_ENTRY=./cmd/server/main.go

.PHONY: build-cli
build-cli:
	$(info #Building...)
	go build -o $(CLI_BIN) $(CLI_ENTRY)

.PHONY: build-server
build-server:
	$(info #Building...)
	go build -o $(SERVER_BIN) $(SERVER_ENTRY)

.PHONY: run-db
run-db:
	$(info #Running...)
	go run $(SERVER_ENTRY)

.PHONY: run-cli
run-cli:
	$(info #Running...)
	go run $(CLI_ENTRY)

.PHONY: field-alignment
field-alignment:
	fieldalignment -fix ./...