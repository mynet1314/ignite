package models

import "time"

type Donate struct {
	Id         int64 `xorm:"pk autoincr notnull"`
	Nickname   string
	DonateType int
	Month      int
	Created    time.Time `xorm:"created"`
}
