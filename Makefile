include scripts/*.mk
BIN=./bin/cmd
ENTRY=./cmd/main.go

.PHONY: build
build:
	$(info #Building...)
	go build -o $(BIN) $(ENTRY)

.PHONY: run
run-db:
	$(info #Running...)
	go run $(ENTRY)

.PHONY: build-image
build-image:
	@docker build -t ${IMAGE_NAME}:${IMAGE_TAG} -f Dockerfile .


.PHONY: field-alignment
field-alignment:
	fieldalignment -fix ./...