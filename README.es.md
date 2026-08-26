<!-- ══════════════════════════ PORTADA ══════════════════════════ -->
<div align="center">
  <img src="docs/title-banner.svg" width="100%" alt="ttlcache"/>
</div>

<!-- ══════════════════════ IDIOMAS / LANGUAGES ══════════════════════ -->
<div align="center">
<a href="README.md"><img src="https://img.shields.io/badge/Português-555555?style=for-the-badge" alt="Português"/></a>
<a href="README.en.md"><img src="https://img.shields.io/badge/English-555555?style=for-the-badge" alt="English"/></a>
<a href="README.es.md"><img src="https://img.shields.io/badge/Español-1987F0?style=for-the-badge" alt="Español"/></a>
</div>

<div align="center">
  <img src="assets/banner.svg" width="100%" alt="ttlcache"/>
</div>

<h1 align="center">ttlcache</h1>
<p align="center"><em>Caché clave-valor en memoria, thread-safe, con TTL por clave, expuesto vía HTTP</em></p>
<p align="center"><strong>PUT/GET/DELETE en /kv/{key} → janitor en background → stats de hit/miss</strong></p>

<div align="center">
<a href="https://github.com/geoggrigori/ttlcache/actions/workflows/ci.yml"><img src="https://github.com/geoggrigori/ttlcache/actions/workflows/ci.yml/badge.svg" alt="CI"/></a>
<img src="https://img.shields.io/badge/Go-1.22%2B-00ADD8?style=flat-square&logo=go&logoColor=white" alt="go"/>
<img src="https://img.shields.io/badge/dependencies-zero-4A1E86?style=flat-square" alt="zero deps"/>
<img src="https://img.shields.io/badge/License-MIT-2E7D32?style=flat-square" alt="license"/>
</div>

<div align="center">
<a href="#acerca-de"><img src="https://img.shields.io/badge/▸_ACERCA_DE-1987F0?style=for-the-badge" alt="acerca"/></a>
<a href="#arquitectura"><img src="https://img.shields.io/badge/▸_ARQUITECTURA-000000?style=for-the-badge" alt="arquitectura"/></a>
<a href="#api-http"><img src="https://img.shields.io/badge/▸_API_HTTP-1987F0?style=for-the-badge" alt="api"/></a>
<a href="#uso"><img src="https://img.shields.io/badge/▸_USO-000000?style=for-the-badge" alt="uso"/></a>
</div>

<br/>

> ⚡ **Cero dependencias** — 100% biblioteca estándar de Go. Nada que descargar, nada que mantener actualizado.

## Acerca de

**ttlcache** es un caché clave-valor en memoria, thread-safe, con **TTL por clave**, servido vía HTTP. Construido enteramente sobre la standard library de Go.

**Destacados:**
- **TTL por clave** — cada entrada expira independientemente; las entradas expiradas se leen como ausentes.
- **Thread-safe** — el store está protegido por `sync.RWMutex` y pasa `go test -race`.
- **Janitor en background** — una goroutine evicta periódicamente las claves expiradas, así la memoria no crece sin límite.
- **Stats de hit/miss** — rastreados con contadores atómicos, expuestos como JSON.
- **API HTTP minúscula** — `PUT`/`GET`/`DELETE` en `/kv/{key}`, más `/keys`, flush y `/stats`, usando el `ServeMux` basado en patrones de Go 1.22+.

## Arquitectura

```mermaid
flowchart LR
    client([Cliente HTTP])

    subgraph srv[net/http ServeMux]
        put[PUT /kv/key]
        get[GET /kv/key]
        del[DELETE /kv/key]
        keys[GET /keys]
        flush[DELETE /kv]
        stats[GET /stats]
    end

    subgraph store[Store - sync.RWMutex]
        m[(key -> value, expiresAt)]
        counters[hits / misses<br/>contadores atómicos]
    end

    janitor[[Goroutine janitor<br/>evicta claves expiradas]]

    client --> put --> m
    client --> get --> m
    get -. hit/miss .-> counters
    client --> del --> m
    client --> keys --> m
    client --> flush --> m
    client --> stats --> counters
    stats --> m
    janitor -- ticker --> m
```

## Build & run

```sh
go build ./...
go run .            # levanta el servidor en :8080
```

## API HTTP

| Método | Ruta | Descripción |
|---|---|---|
| `PUT` | `/kv/{key}` | Guarda el cuerpo de la petición. TTL vía `?ttl=` o `X-TTL`. |
| `GET` | `/kv/{key}` | Devuelve el valor, o `404` si está ausente/expirado. |
| `DELETE` | `/kv/{key}` | Elimina la clave. Siempre `204`. |
| `GET` | `/keys` | Array JSON de las claves actualmente no expiradas. |
| `DELETE` | `/kv` | Limpia todo el caché. Siempre `204`. |
| `GET` | `/stats` | Snapshot JSON: `items`, `hits`, `misses`. |

```sh
curl -X PUT "http://localhost:8080/kv/greeting?ttl=30s" --data "hello"
curl "http://localhost:8080/kv/greeting"        # hello
curl -X DELETE "http://localhost:8080/kv/greeting"
curl "http://localhost:8080/keys"               # ["greeting","session"]
curl "http://localhost:8080/stats"              # {"items":1,"hits":3,"misses":1}
```

## Configuración

| Flag | Env | Predeterminado | Descripción |
|---|---|---|---|
| `-port` | `PORT` | `8080` | Puerto TCP |
| `-ttl` | `TTL` | `1m` | TTL predeterminado cuando la petición omite uno |
| `-janitor-interval` | `JANITOR_INTERVAL` | `30s` | Frecuencia de evicción del janitor |

## Uso

```sh
go test ./...          # todas las pruebas
go test -race ./...    # bajo el detector de race
```

La suite cubre set/get, expiración (vía reloj inyectable), delete, evicción del janitor, `Set`/`Get` concurrente bajo `-race`, y los handlers HTTP vía `httptest`.

## Licencia

[MIT](LICENSE).

<div align="center">
  <img src="https://file.loading.io/color/feature/thumb/Blues-8.png?" width="100%" height="10px" alt="divider"/>
</div>

<p align="center"><sub>Desarrollado por <strong><a href="https://github.com/geoggrigori">Grigori</a></strong> · 2026</sub></p>
