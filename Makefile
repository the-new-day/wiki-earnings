mocks:
	go tool mockery
.PHONY: mocks

test:
	go test -v -race ./...
.PHONY: test

up:
	docker compose up --build
.PHONY: up

down:
	docker compose down
.PHONY: down

seed:
	docker compose --profile migrations run --rm migrator
	docker compose --profile seed run --rm --build seed
.PHONY: seed

migrate-up:
	docker compose --profile migrations run --rm migrator
.PHONY: migrate-up

migrate-down:
	docker compose --profile migrations run --rm migrator down 1
.PHONY: migrate-down

migrate-down-all:
	docker compose --profile migrations run --rm migrator down -all
.PHONY: migrate-down-all

migrate-create:
	docker compose --profile migrations run --rm --entrypoint migrate migrator create -ext sql -dir /migrations -seq $(name)
.PHONY: migrate-create

migrate-version:
	docker compose --profile migrations run --rm migrator version
.PHONY: migrate-version

up-with-migrations:
	docker compose --profile migrations run --rm migrator
	docker compose up --build
.PHONY: up-with-migrations
