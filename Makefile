# --- 0. DOMINIO DE CATÁLOGOS GLOBALES (PK/SK) ---
build-ListCatalogsFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./catalogs/list

build-CreateCatalogFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./catalogs/create

# --- 1. DOMINIO DE ENTIDADES (CLIENTES Y PROVEEDORES UNIFICADOS) ---
build-ListEntitiesFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./entities/list

build-GetEntityFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./entities/get

build-CreateEntityFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./entities/create

build-GetEntityHistoryFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./entities/history

# --- 2. DOMINIO DE PRODUCTOS E INVENTARIO ---
build-ListProductsFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./products/list

build-GetProductFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./products/get

build-SearchProductsFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./products/search

build-CreateProductFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./products/create

build-DeleteProductFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./products/delete

# --- 3. DOMINIO DE PEDIDOS (VENTAS Y COMPRAS UNIFICADAS) ---
build-ListOrdersFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./orders/list

build-GetOrderFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./orders/get

build-SearchOrdersFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./orders/search

build-CreateOrderFunction:
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o $(ARTIFACTS_DIR)/bootstrap ./orders/create