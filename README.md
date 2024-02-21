/build for docker and other infra
/cmd/itchan for short main.go func
/config for config files
/internal/config for config parsers etc
/internal/hander for web request handlers
/internal/middleware for handling middleware
/internal/models for objects reusable in several other folders (user/thread etc)
/internal/scripts basically utils
/internal/services - business logic. All inputs already validated
/internal/storage - wrapper for storage (postgres in our case)
