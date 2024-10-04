package domain

type Board struct {
	Name      string
	ShortName string
	Threads   []*Thread
	CreatedAt int64
}
