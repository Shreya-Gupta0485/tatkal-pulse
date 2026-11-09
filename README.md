# 🚄 Tatkal-Pulse // Distributed High-Concurrency Booking Engine

[![Go Version](https://img.shields.io/badge/Go-1.22+-00ADD8?style=flat&logo=go)](https://go.dev/)
[![Redis](https://img.shields.io/badge/Redis-In--Memory%20Atomic-DC382D?style=flat&logo=redis)](https://redis.io/)
[![Latency](https://img.shields.io/badge/p99%20Latency-%3C15ms-brightgreen)](#benchmarks)
[![Zero Oversell](https://img.shields.io/badge/Overselling-0%20Guaranteed-success)](#concurrency-guarantees)

> A distributed, low-latency reservation engine built in **Go** and **Redis Lua**, designed to eliminate race conditions, prevent database lock contention, and guarantee **zero oversells** during high-concurrency peak bursts (such as the 10:00:00 AM IRCTC Tatkal ticket release).

---

## ⚡ The Problem: Tatkal Race Conditions

During peak ticketing events (like Indian Railways Tatkal at 10:00 AM), thousands of passengers click **"Book Now"** at the exact same millisecond for a handful of available berths.

In conventional architectures:
1. **Race Conditions:** Two concurrent requests check available inventory (`SELECT seats FROM trains WHERE id=22436`). Both read `seats = 1` before either updates, resulting in **negative inventory (overselling)**.
2. **Database Deadlocks:** Locking rows (`SELECT ... FOR UPDATE`) serializes transactions, causing connection pool exhaustion, multi-second tail latency, and gateway timeouts (`504 Gateway Timeout`).

---

## 🏛️ System Architecture

Tatkal-Pulse resolves this by completely decoupling the **Hot In-Memory Reservation Path** (<10ms) from the **Cold Persistence Path** (asynchronous disk writes):