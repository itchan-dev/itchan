// Package api defines the HTTP request and response types for the backend REST API.
//
// These types serve as the shared contract between the backend (which serializes them)
// and the frontend apiclient (which deserializes them). All types are JSON-encoded.
//
// # Design rules
//
// A type belongs here only if it adds value over the domain types in shared/domain:
//
//   - Request DTOs always belong here — they carry JSON tags, validation rules,
//     and field shapes that differ from domain creation types.
//
//   - Response types belong here when they combine or reshape domain data:
//     [BoardListResponse] wraps a slice (JSON requires an object, not a bare array),
//     [LoginResponse] adds an access token, [CreateMessageResponse] pairs an ID with
//     its page number, [BlacklistResponse] and [InviteListResponse] add pagination.
//
//   - Pure domain wrappers that add no fields are omitted — the handler returns
//     the domain type directly, and the apiclient decodes into it directly.
//
// # Cross-cutting concern
//
// Because both services compile against shared/domain, this package does not provide
// a versioning boundary between them. If the services were ever split into independently
// deployed binaries with external clients, response wrappers should be reintroduced
// to decouple the API contract from internal domain evolution.
package api
