package domain

import (
	"time"
)

// to iterate thru layers: handler -> service -> storage
type MessageCreationData struct {
	Board       BoardShortName
	ThreadId    MsgId
	Author      User
	Text        MsgText
	CreatedAt   *time.Time
	Attachments *Attachments
	ReplyTo     *Replies
}

type MessageMetadata struct {
	Board      BoardShortName
	ThreadId   ThreadId
	Id         MsgId
	Author     User
	Op         bool
	Ordinal    int
	Replies    Replies
	CreatedAt  time.Time
	ModifiedAt time.Time
}

type Message struct {
	MessageMetadata
	Text        string
	Attachments *Attachments
	Replies     Replies
}

type Reply struct {
	Board        BoardShortName
	FromThreadId ThreadId
	ToThreadId   ThreadId
	From         MsgId
	To           MsgId
	CreatedAt    time.Time
}

// // Value Marshal
// func (r Replies) Value() (driver.Value, error) {
// 	var jsonData [][]byte
// 	for _, r := range r {
// 		data, err := json.Marshal(r) // Marshal each Reply to JSON bytes
// 		if err != nil {
// 			return nil, err
// 		}
// 		jsonData = append(jsonData, data)
// 	}
// 	return pq.Array(jsonData), nil
// }

// // Scan Unmarshal
// func (r *Replies) Scan(value interface{}) error {
// 	b, ok := value.([][]byte)
// 	if !ok {
// 		return errors.New("type assertion to [][]byte failed")
// 	}
// 	var replies []Reply
// 	for _, val := range b {
// 		var reply Reply
// 		if err := json.Unmarshal(val, &reply); err != nil {
// 			return err
// 		}
// 		r = append(r, reply)
// 	}
// 	r(replies)

// 	return json.Unmarshal(b, &r)
// }
