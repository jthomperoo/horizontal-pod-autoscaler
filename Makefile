REGISTRY = jthomperoo
NAME = horizontal-pod-autoscaler
VERSION = latest

default:
	@echo "=============Building============="
	CGO_ENABLED=0 GOOS=linux go build -o dist/$(NAME) .
	cp LICENSE dist/LICENSE

test:
	@echo "=============Running tests============="
	go test ./... -cover -coverprofile coverage.out

lint:
	@echo "=============Linting============="
	staticcheck ./...

format:
	@echo "=============Formatting============="
	gofmt -s -w .
	go mod tidy

docker: default
	@echo "=============Building docker images============="
	docker build -t $(REGISTRY)/$(NAME):$(VERSION) .

doc:
	@echo "=============Serving docs============="
	mkdocs serve

coverage:
	@echo "=============Loading coverage HTML============="
	go tool cover -html=coverage.out
