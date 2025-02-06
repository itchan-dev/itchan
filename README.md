/shared - for scripts and data structs common for both frontend and backend
/backend/internal/handlers - handle http request and pass it to service
/backend/internal/services - business logic, get request from http and do something with storage
/backend/internal/storage - all interactions with databases. Gets called from services

all errors propagated to top (handlers) level where they linked with status codes. default status code is internal service error. you can customize errors with ErrorWithStatusCode

log errors only once, on the lowest level they occured in internal package (that means log every time you catch error from external code or builtin function)

on handlers level pass every internal error to writeErrorAndStatusCode. Internal error in this context - error in internal module beside "handlers" module

handlers layer should parse request and get arguments for service layer. handlers layers shouldnt validate value of arguments, only ensure that the arguments are presented and have the correct type. Handlers layer also do some web-related work (cookies, http error status handling etc)

service layer contains business logic: validate input arguments, gather data from different resources, transform data etc

storage package interact with different storages and provide data to service level

all layers beside handlers should communicate via models defined in domain package

attachments is file pathes to images/videos on local hard drive