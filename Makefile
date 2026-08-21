.PHONY: start build docker-env docker-config docker-up docker-ps docker-logs docker-test docker-down

NOW = $(shell date -u '+%Y%m%d%I%M%S')

RELEASE_VERSION = v10.1.0

APP 			= xtrading
SERVER_BIN  	= ${APP}
GIT_COUNT 		= $(shell git rev-list --all --count)
GIT_HASH        = $(shell git rev-parse --short HEAD)
RELEASE_TAG     = $(RELEASE_VERSION).$(GIT_COUNT).$(GIT_HASH)

CONFIG_DIR       = ./configs
CONFIG_FILES     = dev
STATIC_DIR       = ./web/dist
START_ARGS       = -d $(CONFIG_DIR) -c $(CONFIG_FILES) -s $(STATIC_DIR)
DOCKER_ENV       = docker/.env
DOCKER_COMPOSE   = docker compose --env-file $(DOCKER_ENV)

all: start

web-install:
	@cd web && npm install

web-dev:
	@cd web && npm run dev

web-build:
	@cd web && npm run build

start:
	@go run -ldflags "-X main.VERSION=$(RELEASE_TAG)" main.go start $(START_ARGS)

build: web-build
	@go build -ldflags "-w -s -X main.VERSION=$(RELEASE_TAG)" -o $(SERVER_BIN)

build-linux:
	CGO_ENABLED=1 GOOS=linux GOARCH=amd64 CC="zig cc -target x86_64-linux-musl" CXX="zig c++ -target x86_64-linux-musl" CGO_CFLAGS="-D_LARGEFILE64_SOURCE" go build -ldflags "-w -s -X main.VERSION=$(RELEASE_TAG)" -o $(SERVER_BIN)_linux_amd64

# go install github.com/google/wire/cmd/wire@v0.7.0
wire:
	@wire gen ./internal/wirex

# go install github.com/swaggo/swag/cmd/swag@latest
swagger:
	@swag init --parseDependency --generalInfo ./main.go --output ./internal/swagger

# https://github.com/OpenAPITools/openapi-generator
openapi:
	docker run --rm -v ${PWD}:/local openapitools/openapi-generator-cli generate -i /local/internal/swagger/swagger.yaml -g openapi -o /local/internal/swagger/v3

clean:
	rm -rf data $(SERVER_BIN)

serve: build
	./$(SERVER_BIN) start $(START_ARGS)

serve-d: build
	./$(SERVER_BIN) start $(START_ARGS) --daemon

stop:
	./$(SERVER_BIN) stop

docker-env:
	@if [ ! -f $(DOCKER_ENV) ]; then cp docker/.env.example $(DOCKER_ENV); fi
	@echo "Docker environment: $(DOCKER_ENV)"

docker-config: docker-env
	@$(DOCKER_COMPOSE) config

docker-up: docker-env
	@$(DOCKER_COMPOSE) up --build -d

docker-ps: docker-env
	@$(DOCKER_COMPOSE) ps

docker-logs: docker-env
	@$(DOCKER_COMPOSE) logs -f --tail=200 app mysql redis

docker-test: docker-env
	@set -a; . $(DOCKER_ENV); set +a; curl --fail --silent --show-error "http://127.0.0.1:$${APP_PORT:-8000}/health"
	@echo

docker-down: docker-env
	@$(DOCKER_COMPOSE) down --remove-orphans
