# shared/api

Wire-format DTOs shared between the backend API and frontend apiclient.

## Rules

### 1. Create an api type when both sides encode/decode it

If the backend marshals a response and the frontend (or any other client) unmarshals it, define a named type here. No inline anonymous structs, no `map[string]T` literals on either side.

```go
// Good
writeJSON(w, api.LastModifiedResponse{LastModifiedAt: lastModified})

var result api.LastModifiedResponse
utils.Decode(resp.Body, &result)

// Bad — fields can drift silently
writeJSON(w, map[string]time.Time{"last_modified_at": lastModified})

var result struct { LastModifiedAt time.Time `json:"last_modified_at"` }
```

### 2. Don't wrap domain types — decode them directly

If the response *is* a domain type (or a slice of one), serialize/deserialize it directly. An api type that only wraps a domain type adds indirection with no benefit.

```go
// Good — domain type used directly on the wire
writeJSON(w, []domain.Message{...})

var messages []domain.Message
utils.Decode(resp.Body, &messages)

// Bad — pointless wrapper
type UserActivityResponse struct { Messages []domain.Message }
```

> **Trade-off:** this couples the wire format to the domain model. If a domain type gains a sensitive internal field with JSON tags, it will be exposed. Add an api type if you need to diverge.

### 3. No response body when status is enough

Mutations that convey no information beyond success (e.g. delete, logout, revoke) return only an HTTP status code. No `{"message": "done"}` bodies.

```go
// Good
w.WriteHeader(http.StatusOK)

// Bad
writeJSON(w, map[string]string{"message": "User blacklisted successfully"})
```
