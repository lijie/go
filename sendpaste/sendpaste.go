package sendpaste

import (
	"time"
)

type SendData struct {
	Data     []byte
	FileName string
}

type PasteData struct {
	Data       []byte
	FileName   string
	CreateTime time.Time
	ID         int64
}
