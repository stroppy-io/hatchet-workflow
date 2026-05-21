package protohelp

import (
	"github.com/samber/lo"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/anypb"
)

func ProtoNew[T proto.Message]() (model T) {
	return model.ProtoReflect().Type().New().Interface().(T)
}

func ProtoFullName[T proto.Message]() string {
	var m T
	return string(m.ProtoReflect().Descriptor().FullName())
}

func ProtoName[T proto.Message]() string {
	var m T
	return string(m.ProtoReflect().Descriptor().Name())
}

func MustAny[T proto.Message](v T) *anypb.Any {
	return lo.Must(anypb.New(v))
}

type IntoPb[T proto.Message] interface {
	IntoPb() T
}

func PlainToPb[P proto.Message, R IntoPb[P]](msg R) P {
	return msg.IntoPb()
}

func MapPlainToPb[P proto.Message, R IntoPb[P]](msgs []R) []P {
	return lo.Map(msgs, func(msg R, _ int) P { return msg.IntoPb() })
}
