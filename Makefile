.PHONY: help gateway converter auth comfyui pdf build-all run-gateway run-converter mirror run-all

help:
	@echo "Доступные команды:"
	@echo "  make gateway          - запустить API Gateway"
	@echo "  make converter        - запустить Converter Service"
	@echo "  make auth             - запустить Auth Service"
	@echo "  make comfyui          - запустить ComfyUI Service"
	@echo "  make pdf              - запустить PDF Service"
	@echo "  make build-all        - собрать все сервисы"
	@echo "  make run-all          - запустить все сервисы (фоново)"
	@echo "  make mirror           - отразить координаты X в test.json"

build-all:
	go build -o bin/gateway ./cmd/gateway
	go build -o bin/converter ./cmd/converter
	go build -o bin/auth ./cmd/auth
	go build -o bin/mirror ./cmd/mirror

mirror:
	go run ./cmd/mirror -i test.json -o test.json

gateway:
	PORT=3000 CONVERTER_URL=http://localhost:3001 AUTH_URL=http://localhost:3002 PDF_SERVICE_URL=http://localhost:3004 go run ./cmd/gateway/main.go

converter:
	PORT=3001 go run ./cmd/converter/main.go

auth:
	PORT=3002 AUTH_DB_PATH=data/db/auth.db go run ./cmd/auth/main.go

comfyui:
	@test -d services/comfyui/venv || (cd services/comfyui && python -m venv venv && venv/bin/pip install -r requirements.txt)
	cd services/comfyui && venv/bin/uvicorn main:app --port 3003 --reload

pdf:
	@test -d cmd/pdf_service/venv || (cd cmd/pdf_service && python -m venv venv && venv/bin/pip install -r requirements.txt)
	cd cmd/pdf_service && PORT=3004 AUTH_SERVICE_URL=http://localhost:3002 venv/bin/python main.py

run-all:
	@echo "Запуск Converter Service..."
	@PORT=3001 go run ./cmd/converter/main.go &
	@sleep 2
	@echo "Запуск Auth Service..."
	@PORT=3002 AUTH_DB_PATH=data/db/auth.db go run ./cmd/auth/main.go &
	@sleep 2
	@echo "Запуск PDF Service..."
	@test -d cmd/pdf_service/venv || (cd cmd/pdf_service && python -m venv venv && venv/bin/pip install -r requirements.txt)
	@cd cmd/pdf_service && PORT=3004 AUTH_SERVICE_URL=http://localhost:3002 venv/bin/python main.py &
	@sleep 2
	@echo "Запуск API Gateway..."
	@PORT=3000 CONVERTER_URL=http://localhost:3001 AUTH_URL=http://localhost:3002 PDF_SERVICE_URL=http://localhost:3004 go run ./cmd/gateway/main.go
