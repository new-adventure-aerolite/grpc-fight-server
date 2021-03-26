.PHONY: build run docker test clean
build:
	go build -o main main.go

run:
	@go run main.go

docker:
	docker build -t fight-server:latest .

test:
	go test -v ./...

clean:
	@rm -rf ./fight-server
