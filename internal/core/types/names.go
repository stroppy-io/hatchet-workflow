package types

type WorkerName string

func (w WorkerName) String() string {
	return string(w)
}
