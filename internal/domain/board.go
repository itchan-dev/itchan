package domain

type timestamp int64

type Board struct {
	Name      string
	ShortName string
	Threads   []Thread
	CreatedAt timestamp
}
