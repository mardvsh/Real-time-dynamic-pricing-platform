.PHONY: up down test build k8s-apply

up:
	docker compose up --build

down:
	docker compose down -v

test:
	go test ./... -v

build:
	go build ./...

k8s-apply:
	kubectl apply -f deploy/k8s/namespace.yaml
	kubectl apply -f deploy/k8s/infra.yaml
	kubectl apply -f deploy/k8s/apps.yaml
	kubectl apply -f deploy/k8s/gateway.yaml
