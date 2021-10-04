.PHONY: build
build:
	@go build -o gcal-fetch-events .
	@mv gcal-fetch-events ~/go/bin
