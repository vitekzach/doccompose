BINARY := doccompose
CMD := .

.PHONY: build run clean

build:
	go build -o $(BINARY) $(CMD)

run:
	go run $(CMD) --podmanmode

clean:
	rm -f $(BINARY)
