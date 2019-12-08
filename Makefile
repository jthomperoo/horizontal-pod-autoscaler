REGISTRY = jthomperoo
NAME = horizontal-pod-autoscaler
VERSION = latest

default: vendor
	@echo "=============Building============="
	CGO_ENABLED=0 GOOS=linux go build -mod vendor -o dist/$(NAME) ./cmd/horizontal-pod-autoscaler
	cp LICENSE dist/LICENSE

unittest: vendor
	@echo "=============Running unit tests============="
	go test ./...  -mod=vendor -cover -coverprofile unit_cover.out --tags=unit

lint: vendor
	@echo "=============Linting============="
	go list -mod=vendor ./... | grep -v /vendor/ | xargs -L1 golint -set_exit_status

docker: default
	@echo "=============Building docker images============="
	docker build -t $(REGISTRY)/$(NAME):$(VERSION) .

vendor:
	go mod vendor