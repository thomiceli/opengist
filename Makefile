.PHONY: all install build_frontend build_backend build build_docker watch_frontend watch_backend watch clean clean_docker

# Specify the name of your Go binary output
BINARY_NAME := opengist

all: install build

install:
	@echo "Installing NPM dependencies..."
	@npm ci || (echo "Error: Failed to install NPM dependencies." && exit 1)
	@echo "Installing Go dependencies..."
	@go mod download || (echo "Error: Failed to install Go dependencies." && exit 1)

build_frontend:
	@echo "Building frontend assets..."
	./node_modules/.bin/vite build

build_backend:
	@echo "Building Opengist binary..."
	go build -tags fs_embed -o $(BINARY_NAME) .

build: build_frontend build_backend

build_docker:
	@echo "Building Docker image..."
	docker build -t $(BINARY_NAME):latest .

watch_frontend:
	@echo "Building frontend assets..."
	./node_modules/.bin/vite dev --port 16157

watch_backend:
	@echo "Building Opengist binary..."
	DEV=1 ./node_modules/.bin/nodemon --watch '**/*' -e html,yml,go,js --signal SIGTERM --exec 'go' run .

watch:
	@bash ./watch.sh

clean:
	@echo "Cleaning up build artifacts..."
	@rm -f $(BINARY_NAME) public/manifest.json
	@rm -rf public/assets

clean_docker:
	@echo "Cleaning up Docker image..."
	@docker rmi $(BINARY_NAME)
