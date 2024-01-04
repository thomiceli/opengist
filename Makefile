.PHONY: all all_crosscompile install build_frontend build_backend build build_crosscompile build_docker watch_frontend watch_backend watch clean clean_docker check_changes go_mod fmt test

# Specify the name of your Go binary output
BINARY_NAME := opengist

all: clean install build

all_crosscompile: clean install build_frontend build_crosscompile

install:
	@echo "Installing NPM dependencies..."
	@npm ci || (echo "Error: Failed to install NPM dependencies." && exit 1)
	@echo "Installing Go dependencies..."
	@go mod download || (echo "Error: Failed to install Go dependencies." && exit 1)

build_frontend:
	@echo "Building frontend assets..."
	npx vite -c public/vite.config.js build
	@EMBED=1 npx postcss 'public/assets/embed-*.css' -c public/postcss.config.js --replace # until we can .nest { @tailwind } in Sass

build_backend:
	@echo "Building Opengist binary..."
	go build -tags fs_embed -o $(BINARY_NAME) .

build: build_frontend build_backend

build_crosscompile:
	@bash ./scripts/build-all.sh

build_docker:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .

watch_frontend:
	@echo "Building frontend assets..."
	npx vite -c public/vite.config.js dev --port 16157

watch_backend:
	@echo "Building Opengist binary..."
	OG_DEV=1 npx nodemon --watch '**/*' -e html,yml,go,js --signal SIGTERM --exec 'go run . --config config.yml'

watch:
	@bash ./scripts/watch.sh

clean:
	@echo "Cleaning up build artifacts..."
	@rm -f $(BINARY_NAME) public/manifest.json
	@rm -rf public/assets build

clean_docker:
	@echo "Cleaning up Docker image..."
	@docker rmi $(BINARY_NAME)

check_changes:
	@echo "Checking for changes..."
	@git --no-pager diff --exit-code || (echo "There are unstaged changes detected." && exit 1)

go_mod:
	@go mod download
	@go mod tidy

fmt:
	@go fmt ./...

test:
	@go test ./... -p 1
