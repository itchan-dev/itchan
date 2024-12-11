/shared - for scripts and data structs common for both frontend and backend
/backend/internal/handlers - handle http request and pass it to service
/backend/internal/services - business logic, get request from http and do something with storage
/backend/internal/storage - all interactions with databases. Gets called from services
