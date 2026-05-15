package middleware

import (
	descpb "google.golang.org/protobuf/types/descriptorpb"

	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/reflect/protoregistry"
)

func BuildIdempotencyRegistry() map[string]bool {
	reg := map[string]bool{}
	protoregistry.GlobalFiles.RangeFiles(func(fd protoreflect.FileDescriptor) bool {
		services := fd.Services()
		for i := 0; i < services.Len(); i++ {
			svc := services.Get(i)
			methods := svc.Methods()
			for j := 0; j < methods.Len(); j++ {
				m := methods.Get(j)
				opts, ok := m.Options().(*descpb.MethodOptions)
				if !ok {
					continue
				}
				if opts.GetIdempotencyLevel() == descpb.MethodOptions_IDEMPOTENT {
					proc := "/" + string(svc.FullName()) + "/" + string(m.Name())
					reg[proc] = true
				}
			}
		}
		return true
	})
	return reg
}
