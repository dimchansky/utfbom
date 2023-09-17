.PHONY: test test_race test_cover fmt vet lint fix

test:
	@go test .

test_race:
	@go test -race .

test_cover:
	@go test -cover -covermode=atomic .

fmt:
	@goimports -w -local github.com/dimchansky/utfbom .
	@gofumpt -w -extra .

vet:
	@go vet .

lint:
	@golint ./...

fix:
	@go fix .

task: fix fmt vet lint test test_race test_cover
	@echo "running..."
