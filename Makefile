.PHONY: test test_race test_cover fmt vet lint fix

test:
	@go test .

test_race:
	@go test -race .

test_cover:
	@go test -cover -covermode=atomic .

# tested with golangci-lint v2.15.0 (same version as CI)
fmt:
	@golangci-lint fmt

vet:
	@go vet .

lint:
	@golangci-lint run

fix:
	@go fix .

task: fix fmt vet lint test test_race test_cover
	@echo "running..."
