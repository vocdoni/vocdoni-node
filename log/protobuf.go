package log

import (
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/reflect/protoreflect"
)

// FormatProto turns a ProtoMessage into a human-readable string
func FormatProto(arg protoreflect.ProtoMessage) string {
	pj := protojson.MarshalOptions{
		AllowPartial:    true,
		Multiline:       false,
		EmitUnpopulated: true,
	}
	return pj.Format(arg)
}
