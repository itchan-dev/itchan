package domain

type Thread struct {
	Title    string
	Messages []Message // all other metainfo = 1st message metainfo
	Board    *Board
}
