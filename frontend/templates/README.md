# Template Guidelines

## When to Use Partials

### ✅ **DO Use Partials For:**

1. **Truly Reusable Components** - Used in 3+ different templates
   - `error-message` - Used everywhere
   - `success-message` - Used everywhere
   - `post` - Complex component used in board and thread pages
   - `post-header` - Used in multiple post contexts
   - `delete-button` - Used in multiple admin contexts
   - `pagination` - Used in multiple listing pages

2. **Complex Logic** - Components with significant template logic
   - `post` component with conditional rendering
   - `pagination` with complex page number logic

3. **Consistent Styling** - Components that must look identical everywhere
   - `error-message` styling
   - `delete-button` styling

### ❌ **DON'T Use Partials For:**

1. **Page-Specific Components** - Used only in one template
   - Board headers (only in board/thread pages)
   - Auth forms (only in auth pages)
   - Navigation elements specific to one page

2. **Simple HTML** - Basic HTML without complex logic
   - Simple form fields
   - Basic navigation links
   - Page titles and headers

3. **One-Time Use** - Components used only once
   - Admin panel (only in index page)
   - Specific form layouts

## Current Partials

### Core Partials (Keep)
- `error-message` - Error display
- `success-message` - Success display  
- `post` - Complete post structure
- `post-header` - Post header with all elements
- `post-attachments` - File attachments
- `post-body` - Post content
- `delete-button` - Admin delete functionality

### Removed Partials (Moved to Templates)
- `board-header` → Moved to board.html, thread.html
- `thread-nav` → Moved to thread.html
- `bottom-nav` → Moved to thread.html
- `auth-form` → Moved to auth templates
- `auth-footer` → Moved to auth templates
- `admin-panel` → Moved to index.html
- `pagination` → Moved to board.html (only used there)
- `reply-summary` → Moved to board.html (only used there)

## Benefits of This Approach

1. **Better Readability** - Page-specific code stays with the page
2. **Easier Maintenance** - Less jumping between files
3. **Clearer Intent** - Partials are clearly for reuse
4. **Reduced Complexity** - No over-abstraction
5. **Faster Development** - Less cognitive overhead

## Template Structure

Each template should be self-contained for page-specific elements while using partials for truly reusable components. This strikes the right balance between DRY principles and maintainability.
