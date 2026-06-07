# Interseguro – Coding Challenge · División TI · Junio 2024

## Arquitectura de la solución

```
Cliente / Frontend
       │
       │  POST /api/v1/matrix/qr
       ▼
┌─────────────────────────┐      POST /api/v1/statistics
│   Go API  (Fiber v2)    │─────────────────────────────▶ ┌──────────────────────────┐
│   Puerto 3000           │ ◀──────────────────────────── │  Node.js API (Express.js) │
│   QR Decomposition      │        estadísticas           │  Puerto 3001              │
└─────────────────────────┘                               │  Cálculo estadístico      │
                                                          └──────────────────────────┘
```

| Servicio     | Framework    | Puerto | Responsabilidad                          |
|--------------|--------------|--------|------------------------------------------|
| **Go API**   | Fiber v2     | 3000   | Factorización QR (Modified Gram-Schmidt) |
| **Node API** | Express 4.x  | 3001   | Estadísticas sobre matrices Q y R        |

---

## Cómo ejecutar (Docker Compose)

**Prerrequisitos:** Docker Desktop instalado.

```bash
# 1. Descomprimir el proyecto
unzip interseguro-challenge.zip
cd interseguro-challenge

# 2. Levantar ambos servicios (primera vez: descarga imágenes y compila)
docker-compose up --build

# 3. Verificar que están corriendo
curl http://localhost:3000/health   # {"status":"ok","service":"go-api"}
curl http://localhost:3001/health   # {"status":"ok","service":"node-api"}
```

Para bajarlos: `docker-compose down`

---

## Endpoints

### Go API — Factorización QR

```
POST http://localhost:3000/api/v1/matrix/qr
Content-Type: application/json
```

**Body:**
```json
{
  "matrix": [
    [1, 2],
    [3, 4],
    [5, 6]
  ]
}
```

**Respuesta exitosa:**
```json
{
  "success": true,
  "message": "QR decomposition and statistics completed successfully",
  "qr": {
    "Q": [
      [0.16903085,  0.89708523],
      [0.50709255,  0.27602622],
      [0.84515425, -0.34503278]
    ],
    "R": [
      [5.91607978, 7.43735772],
      [0,          0.82807867]
    ]
  },
  "statistics": {
    "success": true,
    "data": {
      "aggregate": {
        "max": 7.43735772,
        "min": -0.34503278,
        "average": 1.97020134,
        "totalSum": 15.76161069,
        "elementCount": 8
      },
      "diagonalAnalysis": {
        "Q": { "isDiagonal": false, "reason": "not square (3×2)" },
        "R": { "isDiagonal": false, "reason": "non-zero off-diagonal element at [0][1] = 7.43735772" }
      }
    }
  }
}
```

### (Opcional) Login — obtener JWT

Activar JWT: cambiar `JWT_ENABLED=true` en `docker-compose.yml`.

```
POST http://localhost:3000/api/v1/auth/login
Content-Type: application/json

{ "username": "admin", "password": "interseguro2024" }
```

Usar el token devuelto en el header: `Authorization: Bearer <token>`

### Node.js API — Estadísticas (llamada interna)

También disponible directamente para pruebas:

```
POST http://localhost:3001/api/v1/statistics
Content-Type: application/json

{ "Q": [[...]], "R": [[...]] }
```

---

## Frontend (opcional)

Abrir `frontend/index.html` directamente en el navegador. No requiere servidor adicional.

> ⚠ Asegurarse de que los servicios Docker estén corriendo en `localhost:3000` y `localhost:3001`.

---

## Ejecución local sin Docker

### Go API
```bash
cd go-api
go mod tidy        # descarga dependencias
go run main.go
```

### Node.js API
```bash
cd node-api
npm install
node index.js
```

---

## Decisiones técnicas

### 1. Factorización QR vs. "rotación"
El enunciado menciona "rotación de la matriz" en la sección de arquitectura, pero **"factorización QR"** en la sección de funcionalidad. Se implementa la **factorización QR** (que es la operación matemática explícita). La matriz **Q** resultante es justamente una rotación ortogonal, lo cual reconcilia ambos términos.

### 2. Algoritmo: Modified Gram-Schmidt
Se usa la variante **modificada** de Gram-Schmidt (en lugar de la clásica) porque ofrece mejor estabilidad numérica al acumular errores de punto flotante en operaciones sucesivas.

### 3. Forma economizada (economy QR)
Para una matriz A de m×n (m ≥ n), se retorna:
- **Q**: m×n (columnas ortonormales)
- **R**: n×n (triangular superior)

Esto es suficiente para reconstruir A = Q·R y es más eficiente que la forma completa (m×m + m×n).

### 4. Comunicación síncrona HTTP
El Go API llama al Node.js API de forma síncrona. Si el servicio de estadísticas no está disponible, retorna igual las matrices QR con un aviso en `message`, garantizando **graceful degradation**.

### 5. JWT opcional
El middleware JWT está desactivado por defecto (`JWT_ENABLED=false`). Se activa sin recompilar código, solo cambiando una variable de entorno. En producción se usaría una base de datos para validar credenciales.

### 6. Redondeo
Valores con valor absoluto < 1e-10 se tratan como cero para evitar ruido de punto flotante en la salida (ej. `1.2345e-16` aparece como `0`).

---

## Estructura del proyecto

```
interseguro-challenge/
├── go-api/
│   ├── main.go          ← API Go completa (QR + JWT + cliente HTTP)
│   ├── go.mod           ← Módulo Go (Fiber v2, golang-jwt)
│   └── Dockerfile       ← Build multi-etapa (builder + alpine)
├── node-api/
│   ├── index.js         ← API Node.js completa (estadísticas)
│   ├── package.json     ← Dependencias (Express, cors)
│   └── Dockerfile       ← Imagen node:20-alpine
├── frontend/
│   └── index.html       ← SPA que consume la Go API (opcional)
├── docker-compose.yml   ← Orquestación de ambos servicios
└── README.md            ← Este archivo
```
