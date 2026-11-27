.PHONY: run test docker-up docker-down

run:
	go run cmd/server/main.go

test:
	go test -v -race ./...

docker-up:
	docker compose up -d

docker-down:
	docker compose down
