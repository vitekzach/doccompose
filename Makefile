BINARY := doccompose
CMD := .

.PHONY: build run clean

build:
	GOOS=linux GOARCH=amd64 go build -o builds/$(BINARY) $(CMD)
	GOOS=windows GOARCH=amd64 go build -o builds/$(BINARY).exe $(CMD)
	GOOS=darwin GOARCH=amd64 go build -o builds/$(BINARY)-macos $(CMD)
	GOOS=linux GOARCH=arm64 go build -o builds/$(BINARY)-arm $(CMD)
	GOOS=windows GOARCH=arm64 go build -o builds/$(BINARY)-arm.exe $(CMD)
	GOOS=darwin GOARCH=arm64 go build -o builds/$(BINARY)-macos-arm $(CMD)

run:
	go run $(CMD) --podmanmode

clean:
	rm -f $(BINARY)
