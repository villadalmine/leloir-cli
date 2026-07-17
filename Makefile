# leloir-cli — build, test y el ciclo de release AUTO-GESTIONADO.
# La versión es la ÚNICA fuente de verdad (archivo VERSION); todo la deriva.
VERSION := $(shell cat VERSION)
BIN     := leloir
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test sync check-sync release-check version help
.DEFAULT_GOAL := help

build: ## Compila el binario (con la versión inyectada)
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) ./cmd/leloir

test: ## Corre los tests
	go test ./...

sync: ## Trae la fuente del canónico (leloir-core/cmd/leloir) a este repo
	bash scripts/sync-from-core.sh

check-sync: ## Falla si la fuente vendorizada quedó drift del canónico
	bash scripts/sync-from-core.sh --check

release-check: test check-sync ## Puerta ANTES de tagear un release: test + sin drift
	@echo "✅ listo para release $(VERSION) — tageá con: git tag $(VERSION) && git push --tags"

version: ## Imprime la versión
	@echo $(VERSION)

help: ## Esta ayuda
	@grep -hE '^[a-z-]+:.*##' $(MAKEFILE_LIST) | sort | awk 'BEGIN{FS=":.*##"}{printf "  \033[36m%-14s\033[0m %s\n",$$1,$$2}'
