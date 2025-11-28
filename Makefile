.PHONY: help gateway converter build-all run-gateway run-converter

help:
	@echo "Доступные команды:"
	@echo "  make gateway          - запустить API Gateway"
	@echo "  make converter        - запустить Converter Service"
	@echo "  make build-all        - собрать все сервисы"
	@echo "  make run-all          - запустить все сервисы (фоново)"

build-all:
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/converter ./cmd/converter

gateway:
	PORT=3000 CONVERTER_URL=http://localhost:3001 go run ./cmd/gateway/main.go

converter:
	PORT=3001 go run ./cmd/converter/main.go

run-all:
	@echo "Запуск Converter Service..."
	@PORT=3001 go run ./cmd/converter/main.go &
	@sleep 2
	@echo "Запуск API Gateway..."
	@PORT=3000 CONVERTER_URL=http://localhost:3001 go run ./cmd/gateway/main.go
