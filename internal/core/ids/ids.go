package ids

import (
	"fmt"
	"strings"

	"github.com/oklog/ulid/v2"
)

type OIDC = string

type Ulid struct {
	Id string
}

func (u *Ulid) GetId() string {
	if u == nil {
		return ""
	}
	return u.Id
}

func NewUlid() *Ulid {
	return &Ulid{
		Id: ulid.Make().String(),
	}
}

func UlidToStr(ulid *Ulid) string {
	if ulid == nil {
		return ""
	}
	return ulid.GetId()
}

func UlidFromString(str string) *Ulid {
	return &Ulid{
		Id: str,
	}
}

func UlidToStrPtr(ulid *Ulid) *string {
	if ulid == nil {
		return nil
	}
	val := ulid.GetId()
	return &val
}

func UlidFromStringPtr(str *string) *Ulid {
	if str == nil {
		return nil
	}
	return UlidFromString(*str)
}

func ExtractFromMediaRoomID(roomId string) (*Ulid, *Ulid, error) {
	parts := strings.Split(roomId, "-")
	if len(parts) != 2 {
		return nil, nil, fmt.Errorf("invalid room id: %s", roomId)
	}
	return UlidFromString(parts[0]), UlidFromString(parts[1]), nil
}

type RunId string

func NewRunId() RunId {
	return RunId(strings.ToLower(ulid.Make().String()))
}

func (id RunId) String() string {
	return string(id)
}

type HatchetRunId string

func NewHatchetRunId() HatchetRunId {
	return HatchetRunId(strings.ToLower(ulid.Make().String()))
}

func (id HatchetRunId) String() string {
	return string(id)
}
