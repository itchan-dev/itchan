package domain

type User struct {
	Email    string
	PassHash []byte
	Id       int64
}
