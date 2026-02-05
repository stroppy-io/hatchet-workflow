package ids

import (
	"strings"

	"github.com/oklog/ulid/v2"
)

type Ulid struct {
	Id string
}

type LowerUlid Ulid

func (u LowerUlid) String() string {
	return strings.ToLower(u.Id)
}

func (u Ulid) String() string {
	return u.Id
}

func (u Ulid) Lower() LowerUlid {
	return LowerUlid(u)
}

func NewUlid() Ulid {
	return Ulid{
		Id: ulid.Make().String(),
	}
}

func UlidToStr(ulid Ulid) string {
	return ulid.String()
}

func UlidFromString(str string) *Ulid {
	return &Ulid{
		Id: str,
	}
}

func Lower(str string) string {
	return strings.ToLower(str)
}

type RunId Ulid

func ParseRunId(str string) RunId {
	return RunId{
		Id: Lower(str),
	}
}
func NewRunId() RunId {
	return RunId(NewUlid().Lower())
}

func (id RunId) String() string {
	return id.Id
}

//type HatchetRunId string
//
//func NewHatchetRunId() HatchetRunId {
//	return HatchetRunId(strings.ToLower(ulid.Make().String()))
//}
//
//func (id HatchetRunId) String() string {
//	return string(id)
//}
