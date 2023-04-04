.PHONY: all install_deps build_frontend build_backend build_docker clean clean_docker

# Specify the name of your Go binary output
BINARY_NAME := opengist

all: install_deps build_frontend build_backend

install_deps:
	@echo "Installing NPM dependencies..."
	@npm ci || (echo "Error: Failed to install NPM dependencies." && exit 1)
	@echo "Installing Go dependencies..."
	@go mod download || (echo "Error: Failed to install Go dependencies." && exit 1)

build_frontend:
	@echo "Building frontend assets..."
	./node_modules/.bin/vite build

build_backend:
	@echo "Building Opengist binary..."
	go build -o $(BINARY_NAME) opengist.go

build_docker:
	@echo "Building Docker image..."
	docker build -t opengist .

clean:
	@echo "Cleaning up build artifacts..."
	@rm -f $(BINARY_NAME) public/manifest.json
	@rm -rf node_modules public/assets

clean_docker:
	@echo "Cleaning up Docker image..."
	@docker rmi opengist