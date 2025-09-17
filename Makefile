.PHONY: build test clean docker-build docker-build-multi docker-build-multi-local docker-build-amd64 docker-build-arm64 docker-build-local docker-push deploy undeploy setup-buildx show-platforms

# Build the application
build:
	go build -o connector .

# Run tests
test:
	go test ./...

# Clean build artifacts
clean:
	rm -f connector

# Build Docker image (single platform)
docker-build:
	docker build -t openwebui-content-sync:latest .

# Build multi-architecture Docker image (builds manifest, no local load)
docker-build-multi-local:
	docker buildx build --platform linux/amd64,linux/arm64 -t openwebui-content-sync:latest .

# Build multi-architecture Docker image and push to registry
docker-build-multi:
	docker buildx build --platform linux/amd64,linux/arm64 -t castaiphil/openwebui-content-sync:latest --push .

# Build for specific platform and load locally (useful for testing)
docker-build-amd64:
	docker buildx build --platform linux/amd64 -t openwebui-content-sync:amd64 --load .

docker-build-arm64:
	docker buildx build --platform linux/arm64 -t openwebui-content-sync:arm64 --load .

# Build for current platform and load locally (useful for testing)
docker-build-local:
	docker buildx build --platform linux/amd64,linux/arm64 -t openwebui-content-sync:latest --load --builder desktop-linux .

# Push Docker image (update registry as needed)
docker-push:
	docker push openwebui-content-sync:latest

# Deploy to Kubernetes
deploy:
	kubectl apply -f k8s/

# Undeploy from Kubernetes
undeploy:
	kubectl delete -f k8s/

# Run locally with config
run:
	./content -config config.yaml

# Run tests with coverage
test-coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

# Format code
fmt:
	go fmt ./...

# Lint code
lint:
	golangci-lint run

# Install dependencies
deps:
	go mod download
	go mod tidy

# Setup Docker buildx for multi-arch builds
setup-buildx:
	docker buildx create --name multiarch --driver docker-container --use
	docker buildx inspect --bootstrap

# Show available platforms
show-platforms:
	docker buildx inspect
