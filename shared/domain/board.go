package domain

import "time"

type Board struct {
	Name      string
	ShortName string
	Threads   []*Thread
	CreatedAt time.Time
}
