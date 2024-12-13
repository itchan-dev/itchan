/shared - for scripts and data structs common for both frontend and backend
/backend/internal/handlers - handle http request and pass it to service
/backend/internal/services - business logic, get request from http and do something with storage
/backend/internal/storage - all interactions with databases. Gets called from services

all errors propagated to handlers level. default status code is internal service error. you can customize errors with ErrorWithStatusCode

log errors only once, on the lowest level they occured in itchan package (that means log every time you catch error from external code or builting function)