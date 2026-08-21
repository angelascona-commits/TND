# 

Documento de Arquitectura y Diseño de Datos \- Tienda Virtual

**Paradigma de Base de Datos:** NoSQL (Clave-Valor / Documentos)  
**Patrón Arquitectónico:** Diseño de Tabla Única (Single-Table Design) con Sobrecarga de Índices

## **1\. Condiciones Iniciales y Objetivos del Diseño**

El diseño de esta base de datos ha sido estructurado para cumplir con los siguientes requerimientos fundacionales del negocio:

> * **Alta Escalabilidad con Consumo Mínimo:** Soportar volúmenes masivos de datos sin degradación del rendimiento. Las lecturas y escrituras operan con complejidad constante O(1) mediante el uso de particiones lógicas, evitando escaneos completos (Scans) que incrementen los costos.  
> * **Estandarización Estricta (Catálogo):** Implementación de una parametrización global que gobierna el comportamiento de las interfaces y valida la entrada de datos, garantizando la integridad referencial sin necesidad de modificar el código fuente.  
> * **Trazabilidad Física y Financiera:** Registro granular de compras a proveedores y ventas a clientes, incluyendo el seguimiento individual de productos mediante números de serie (Serialización).  
> * **Preparación Analítica:** Esta estructura garantiza un histórico de transacciones limpio y serializado, sentando las bases de datos necesarias para futuros despliegues de modelos predictivos de aprovisionamiento autónomo basados en Machine Learning, permitiendo mitigar el quiebre de stock en el comercio minorista considerando restricciones presupuestales.

## **2\. Definición del Esquema Base y GSIs**

El sistema consolida toda la información en una única tabla física, diferenciando las entidades mediante prefijos estandarizados. Se emplean exclusivamente dos Índices Secundarios Globales (GSIs) para resolver consultas transversales sin duplicar costos operativos.

> * **Clave de Partición (PK):** Agrupador principal de la entidad (La "Carpeta").  
> * **Clave de Ordenamiento (SK):** Identificador único, nivel de detalle o variante (El "Documento").  
> * **GSI1 (Índice de Trazabilidad):** Diseñado para consultas operativas inversas.  
> * **GSI2 (Índice de Descubrimiento):** Diseñado para filtros y agrupaciones de catálogo.

## **3\. Modelado de Entidades (Dominios)**

### **3.1 Dominio: Catálogo Global (Parametría)**

Controla las listas desplegables, jerarquías y reglas dinámicas del sistema.

| PK (Agrupador) | SK (Identificador) | Atributos Estándar | Atributos Avanzados (Reglas)   |
| :---- | :---- | :---- | :---- |
| CATALOG\#\<Categoria\> | ITEM\#\<ID\_Item\> | Label (Etiqueta UI), IsActive, SortOrder | Metadata (JSON con reglas de validación), ParentID (Para subcategorías) |

### **3.2 Dominio: Entidades (Clientes y Proveedores)**

Centraliza los perfiles de los actores comerciales.

| PK (Agrupador) | SK (Identificador) | Atributos del Registro   |
| :---- | :---- | :---- |
| USER\#\<DNI\_o\_RUC\> | PROFILE | Nombre, Correo, Direccion, Telefono, Rol (Cliente/Proveedor) |

### **3.3 Dominio: Inventario y Serialización**

Agrupa la información general del producto, sus variantes físicas y el estado individual de cada unidad en almacén.

| PK (Producto) | SK (Detalle/Variante) | GSI1 (Trazabilidad) | GSI2 (Filtros) | Atributos del Registro   |
| :---- | :---- | :---- | :---- | :---- |
| PROD\#\<ID\> | INFO\_BASE |  | GSI2\_PK: CAT\#\<ID\_Cat\> GSI2\_SK: BRAND\#\<ID\_Marca\> | Nombre, Descripcion, Estado |
| PROD\#\<ID\> | SKU\#\<VarianteID\> |  |  | Precio, StockTotal, Color, RAM |
| PROD\#\<ID\> | SN\#\<NumeroSerie\> | GSI1\_PK: SN\#\<NumeroSerie\> |  | SKU\_Asociado, EstadoFisico (Disponible/Vendido), Ubicacion, TrxID\_Origen |

### **3.4 Dominio: Transacciones (Compras y Ventas)**

Registra la operación financiera global y detalla cada unidad física involucrada.

| PK (Operación) | SK (Línea de Detalle) | GSI1 (Dueño) | GSI1 (Fecha) | Atributos del Registro   |
| :---- | :---- | :---- | :---- | :---- |
| TRX\#\<ID\> | HEADER | GSI1\_PK: USER\#\<DNI\_RUC\> | GSI1\_SK: \<Fecha\> | TipoOperacion, Total, EstadoPago |
| TRX\#\<ID\> | DETAIL\#SKU\#\<ID\>\#SN\#\<Serie\> |  |  | Cantidad: 1, PrecioUnitario, Subtotal |
| TRX\#\<ID\> | DETAIL\#SKU\#\<ID\_Generico\> |  |  | Cantidad: N, PrecioUnitario, Subtotal |

*(Nota: La línea genérica se utiliza para productos que no requieren seguimiento por número de serie).*

## **4\. Casos de Uso (Access Patterns)**

El diseño responde de manera óptima a los patrones de lectura y escritura definidos por el negocio:

### **4.1 Patrones de Escritura (Writes)**

> * **Creación de Nuevos Productos:** Se inserta el bloque PROD incluyendo el registro INFO\_BASE y sus variantes SKU. Los identificadores de marca y categoría se extraen previamente del catálogo.  
> * **Nueva Existencia (Ingreso a Almacén):** Se ejecutan dos acciones en una misma transacción atómica:  
>   1\. Se crean los registros individuales SN\# para cada unidad física con estado "Disponible".  
>   2\. Se incrementa el contador StockTotal en el registro SKU correspondiente.  
> * **Nuevas Compras/Ventas:** Se genera un registro HEADER para agrupar la factura/boleta y múltiples registros DETAIL por cada ítem transaccionado. En el caso de ventas, se actualizan simultáneamente los registros SN\# del inventario, cambiando su estado a "Vendido" y vinculándolos al ID de la transacción.

### **4.2 Patrones de Lectura (Queries)**

> * **Carga Dinámica de Vistas (Listas y Reglas):** Búsqueda directa por PK \= CATALOG\#\<Categoria\>. Los resultados se ordenan en memoria utilizando el atributo SortOrder y las validaciones se aplican leyendo el objeto Metadata.  
> * **Búsqueda de Transacciones por ID:** Consulta directa por PK \= TRX\#\<ID\> para obtener instantáneamente la cabecera del comprobante y todos sus ítems al mismo tiempo.  
> * **Historial de Compras/Ventas por Entidad:** Consulta al GSI1 (GSI1\_PK \= USER\#\<DNI\>). Retorna todos los comprobantes asociados al cliente o proveedor, ordenados temporalmente gracias a GSI1\_SK.  
> * **Rastreo de Seriales (Garantías/Auditoría):** Consulta al GSI1 (GSI1\_PK \= SN\#\<NumeroSerie\>). Permite identificar de forma inmediata a qué producto pertenece una caja física y en qué documento exacto se vendió o ingresó.  
> * **Filtros de Inventario (Búsqueda por Tipo/Marca):** Consulta al GSI2 combinando la categoría y la marca (Ej: GSI2\_PK \= CAT\#LAPTOPS y GSI2\_SK \= BRAND\#APPLE). Recupera rápidamente la lista de productos que cumplen con los criterios para ser mostrados en la tienda.