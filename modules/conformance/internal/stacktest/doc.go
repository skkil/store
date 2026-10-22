// Package stacktest starts the pieces the conformance tests run against:
// Postgres with the store migrations and the reference schema
// (Testcontainers), the store service binary built from modules/service as
// a subprocess, and reference product servers in-process. It is never
// linked into a binary.
package stacktest
