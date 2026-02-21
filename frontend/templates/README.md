# Template Guidelines

## Core Principle: Templates Are for Presentation Only

Business logic belongs in Go. Templates should render data, not compute it.

**Go is responsible for:**
- Filtering and categorizing data (e.g. splitting boards into public/corporate)
- Computing derived values (e.g. omitted reply count)
- Dereferencing pointers and constructing URLs
- Determining which data a page needs

**Templates are responsible for:**
- Rendering pre-computed data as HTML
- Conditional display (show/hide elements)
- Iteration over pre-built slices
- Formatting values for display (via template functions)

---

## Passing Data to Templates

### Page data — use typed structs

Each page handler passes a named struct as `.Data`. These are defined in
`frontend/internal/domain/pages.go`. Anonymous inline structs are not used.

```go
// Good
h.renderTemplate(w, r, "board.html", frontend_domain.BoardPageData{
    Board:       renderBoard(board),
    CurrentPage: page,
})
```

### Partial data — two patterns depending on type

**Domain partials** (post, thread) receive a typed struct.
The `post` partial receives `*PostData` built via the `postData` template function:

```html
{{template "post" (postData $message $.Common)}}
```

**UI widget partials** (file-input, delete-button, password-input, message-link)
receive a `dict`. Input IDs, CSS classes, and action URLs are HTML/CSS concerns
that belong in the template, not in Go:

```html
{{template "file-input" dict "InputID" "attachments" "MaxCount" .Common.Validation.MaxAttachmentsPerMessage ...}}
{{template "delete-button" dict "Action" (printf "/%s/delete" .ShortName) ...}}
```

---

## Template Functions

Template functions are for **display formatting only**. They must not encode
business logic or data transformation.

| Function | Purpose |
|---|---|
| `sub`, `add` | Arithmetic (Go templates have no operators) |
| `dict` | Build map for UI widget partials |
| `postData` | Construct typed `PostData` for the `post` partial |
| `bytesToMB` | Format byte count for display |
| `mimeTypeExtensions` | Format MIME types as extensions for display |
| `formatAcceptMimeTypes` | Build HTML `accept` attribute string |
| `thumbDims` | Compute display dimensions preserving aspect ratio |
| `join` | Join strings with separator for display |

---

## When to Use Partials

### Use partials for:
- Components used in **3+ templates** (`post`, `delete-button`, `csrf-field`)
- Components with enough structure to warrant isolation (`post-header`, `post-attachments`)

### Don't use partials for:
- Page-specific HTML used only once (`board-header`, admin panel form)
- Simple HTML without meaningful reuse (`auth-footer`, basic nav links)

### Current partials
- `csrf-field` — hidden CSRF token input
- `error-message` / `success-message` — flash message display
- `post` — complete post (header + attachments + body)
- `post-header` — author, date, id, reply/admin controls
- `post-attachments` — image and video rendering
- `post-body` — quoted post text
- `file-input` — file upload input with hints
- `password-input` — password field with validation hint
- `popup-reply-form` — floating reply form
- `delete-button` / `pin-toggle-button` / `blacklist-button` — admin action forms
- `message-link` — `>>threadId#msgId` reply link
- `agreement-notice` — registration legal notice
