# Tatkal-Pulse // High-Concurrency Seat Booking Engine

A low-latency reservation engine built with **Go (Gin)** and **Redis Lua scripting**, designed to demonstrate how to avoid race conditions and overselling when many users try to book the last seat at the same moment (like the 10:00 AM IRCTC Tatkal release).

> **Scope:** This is a concurrency demo for a single train (`22436`) with a single Tatkal seat. It focuses on the atomic reservation step. It is not a full IRCTC system.

## ⚡ The Problem: Tatkal Race Conditions

During peak ticketing events, many passengers click "Book Now" at the same millisecond for very few seats.

1. **Race conditions:** Two requests read `seats = 1` before either updates it, so both succeed and inventory goes negative (overselling).
2. **Lock contention:** Locking rows (`SELECT ... FOR UPDATE`) serializes requests, exhausts connection pools and increases tail latency.

## 🏛️ Architecture (as implemented)

```
[Browser: two passengers / "Both click at once"]
                   │  POST /api/v1/book
                   ▼
        [Go HTTP API (Gin, goroutine per request)]
                   │  EVALSHA (pre-loaded script)
                   ▼
        [Redis: executes Lua script atomically]
                   │
        ┌──────────┴───────────┐
        ▼                      ▼
 train:<no>:tatkal_seats   train:<no>:passengers
   (seat counter)            (set of booked names)
```

* On startup the server reads `scripts/reserve.lua` and loads it into Redis with `SCRIPT LOAD`, then calls it using `EVALSHA` on every booking.
* The script receives `KEYS = [seatKey, passengersKey]` and `ARGV = [passenger, pnr]`, and returns one of three codes.
* Redis runs a Lua script as a single atomic unit, so no other command can interleave while it executes. This is what prevents two requests from both taking the last seat.

| Script result | HTTP status | `status` field   | Meaning                           |
|---------------|-------------|------------------|-----------------------------------|
| `1`           | 200         | `CONFIRMED`      | Seat allocated, PNR generated     |
| `0`           | 409         | `REGRET`         | No seats left                     |
| `-1`          | 403         | `ALREADY_BOOKED` | Same passenger name already booked |

## 🛡️ What This Demo Guarantees

* **No overselling:** The seat check and decrement happen inside one atomic Lua execution.
* **Duplicate prevention:** A passenger name already stored in the Redis set is rejected with `ALREADY_BOOKED`.
* **No disk I/O on the booking path:** Everything happens in Redis memory.
* **Serialized resolution:** Concurrent requests are resolved one after another inside Redis, in the order Redis receives them. (Order of arrival at Redis depends on network and goroutine scheduling, so this is not a strict real-time FIFO guarantee.)

> Latency numbers shown in the UI are browser round-trip times on localhost. They are not a formal benchmark.

## 📡 API Reference

### 1. Book Tatkal Seat

`POST /api/v1/book` · `Content-Type: application/json`

Request:

```json
{
  "train_no": "22436",
  "passenger": "Shreya"
}
```

Both fields are required. A missing field returns `400 {"error": "Invalid booking payload"}`.

**200 OK – Confirmed**

```json
{
  "status": "CONFIRMED",
  "pnr": "PNR-5ad3044a",
  "berth": "B1 - Berth 42 (Lower)",
  "passenger": "Shreya",
  "message": "🎉 TICKET CONFIRMED (CNF)! Seat allocated."
}
```

**409 Conflict – Exhausted**

```json
{
  "status": "REGRET",
  "passenger": "Amrita",
  "message": "😔 REGRET / NOT AVAILABLE: All Tatkal seats exhausted!"
}
```

**403 Forbidden – Duplicate**

```json
{
  "status": "ALREADY_BOOKED",
  "passenger": "Shreya",
  "message": "⚠️ Duplicate booking detected for passenger."
}
```

**500** is returned with `{"error": "..."}` if the Redis/script call fails.

### 2. Reset Tatkal Quota

`POST /api/v1/reset`

Sets `train:22436:tatkal_seats` back to `1` and clears `train:22436:passengers`. The same reset also runs once automatically at server startup.

```json
{ "status": "RESET", "message": "Tatkal Quota successfully reset to 1 seat!" }
```

### 3. Frontend

`GET /` serves `frontend/index.html`.

## 🖥️ Interactive Simulation Client

`frontend/index.html`, styled in a Royal Blue & White palette:

* **Two passenger cards** with empty, editable name inputs and individual **Book** buttons.
* **"Both click at once"** button that fires both booking requests together using `Promise.all`, so the backend has to resolve the race. Exactly one passenger should get `CONFIRMED` and the other `REGRET`.
* **Reset seat** button that calls `/api/v1/reset` to restore the single seat so you can run the test again.
* Shows round-trip latency for each attempt.

## 🚀 Quickstart

**Prerequisites**

* Go (see `go.mod` for the required version)
* Redis (or Memurai on Windows) running on `localhost:6379`

**Project layout (run from the repo root)**

```
tatkal-pulse/
├── main.go
├── go.mod
├── scripts/
│   └── reserve.lua
└── frontend/
    └── index.html
```

The server reads `scripts/reserve.lua` and `frontend/index.html` using relative paths, so start it from the repo root.

**Run**

```bash
go mod tidy
go run main.go
```

Then open `http://localhost:8081/`.

## 🛠️ Tech Stack

* **Language:** Go
* **Web framework:** Gin
* **Store:** Redis, accessed with `go-redis/v9` (pool size 100)
* **Concurrency control:** Redis Lua script via `EVALSHA`
* **PNR generation:** `google/uuid` (first 8 characters)
* **Frontend:** Vanilla HTML, CSS, JavaScript (`fetch` + `Promise.all`)

## ⚠️ Current Limitations

* Single hardcoded train (`22436`) in the reset logic and the UI.
* Berth is a fixed string (`B1 - Berth 42 (Lower)`), not a real allocation.
* Duplicate detection is by passenger name only.
* No database persistence. State lives only in Redis.
* CORS is open (`*`) for local demo use.

## 🗺️ Roadmap

* **Full reservation lifecycle:** `HELD` (seat reserved with a TTL) → `CONFIRMED` (after payment) or `EXPIRED` (seat auto-released), plus cancellation.
* Waiting list (WL) promotion when a confirmed ticket is cancelled.
* Asynchronous persistence of confirmed bookings to a database via a worker queue.
* Real berth allocation and multi-train support.
* Load test (e.g. `k6` / `hey`) to publish real latency numbers.
