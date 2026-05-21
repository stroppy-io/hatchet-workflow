package protohelp

import "google.golang.org/protobuf/proto"

type EncoderDecoder[T proto.Message] struct {
}

func NewEncoderDecoder[T proto.Message]() *EncoderDecoder[T] {
	return &EncoderDecoder[T]{}
}

func (e *EncoderDecoder[T]) Encode(v T) ([]byte, error) {
	return proto.Marshal(v)
}

func (e *EncoderDecoder[T]) Decode(v []byte) (T, error) {
	t := ProtoNew[T]()
	err := proto.Unmarshal(v, t)
	return t, err
}
