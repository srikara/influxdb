// Code generated by protoc-gen-gogo.
// source: internal/data.proto
// DO NOT EDIT!

/*
Package internal is a generated protocol buffer package.

It is generated from these files:
	internal/data.proto

It has these top-level messages:
	WriteShardRequest
	Field
	Tag
	Point
	WriteShardResponse
*/
package internal

import proto "github.com/gogo/protobuf/proto"
import math "math"

// Reference imports to suppress errors if they are not otherwise used.
var _ = proto.Marshal
var _ = math.Inf

type WriteShardRequest struct {
	ShardID          *uint64  `protobuf:"varint,1,req" json:"ShardID,omitempty"`
	Points           []*Point `protobuf:"bytes,2,rep" json:"Points,omitempty"`
	XXX_unrecognized []byte   `json:"-"`
}

func (m *WriteShardRequest) Reset()         { *m = WriteShardRequest{} }
func (m *WriteShardRequest) String() string { return proto.CompactTextString(m) }
func (*WriteShardRequest) ProtoMessage()    {}

func (m *WriteShardRequest) GetShardID() uint64 {
	if m != nil && m.ShardID != nil {
		return *m.ShardID
	}
	return 0
}

func (m *WriteShardRequest) GetPoints() []*Point {
	if m != nil {
		return m.Points
	}
	return nil
}

type Field struct {
	Name             *string  `protobuf:"bytes,1,req" json:"Name,omitempty"`
	Int32            *int32   `protobuf:"varint,2,opt" json:"Int32,omitempty"`
	Int64            *int64   `protobuf:"varint,3,opt" json:"Int64,omitempty"`
	Float64          *float64 `protobuf:"fixed64,4,opt" json:"Float64,omitempty"`
	Bool             *bool    `protobuf:"varint,5,opt" json:"Bool,omitempty"`
	String_          *string  `protobuf:"bytes,6,opt" json:"String,omitempty"`
	Bytes            []byte   `protobuf:"bytes,7,opt" json:"Bytes,omitempty"`
	XXX_unrecognized []byte   `json:"-"`
}

func (m *Field) Reset()         { *m = Field{} }
func (m *Field) String() string { return proto.CompactTextString(m) }
func (*Field) ProtoMessage()    {}

func (m *Field) GetName() string {
	if m != nil && m.Name != nil {
		return *m.Name
	}
	return ""
}

func (m *Field) GetInt32() int32 {
	if m != nil && m.Int32 != nil {
		return *m.Int32
	}
	return 0
}

func (m *Field) GetInt64() int64 {
	if m != nil && m.Int64 != nil {
		return *m.Int64
	}
	return 0
}

func (m *Field) GetFloat64() float64 {
	if m != nil && m.Float64 != nil {
		return *m.Float64
	}
	return 0
}

func (m *Field) GetBool() bool {
	if m != nil && m.Bool != nil {
		return *m.Bool
	}
	return false
}

func (m *Field) GetString_() string {
	if m != nil && m.String_ != nil {
		return *m.String_
	}
	return ""
}

func (m *Field) GetBytes() []byte {
	if m != nil {
		return m.Bytes
	}
	return nil
}

type Tag struct {
	Key              *string `protobuf:"bytes,1,req" json:"Key,omitempty"`
	Value            *string `protobuf:"bytes,2,req" json:"Value,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *Tag) Reset()         { *m = Tag{} }
func (m *Tag) String() string { return proto.CompactTextString(m) }
func (*Tag) ProtoMessage()    {}

func (m *Tag) GetKey() string {
	if m != nil && m.Key != nil {
		return *m.Key
	}
	return ""
}

func (m *Tag) GetValue() string {
	if m != nil && m.Value != nil {
		return *m.Value
	}
	return ""
}

type Point struct {
	Name             *string  `protobuf:"bytes,1,req" json:"Name,omitempty"`
	Time             *int64   `protobuf:"varint,2,req" json:"Time,omitempty"`
	Fields           []*Field `protobuf:"bytes,3,rep" json:"Fields,omitempty"`
	Tags             []*Tag   `protobuf:"bytes,4,rep" json:"Tags,omitempty"`
	XXX_unrecognized []byte   `json:"-"`
}

func (m *Point) Reset()         { *m = Point{} }
func (m *Point) String() string { return proto.CompactTextString(m) }
func (*Point) ProtoMessage()    {}

func (m *Point) GetName() string {
	if m != nil && m.Name != nil {
		return *m.Name
	}
	return ""
}

func (m *Point) GetTime() int64 {
	if m != nil && m.Time != nil {
		return *m.Time
	}
	return 0
}

func (m *Point) GetFields() []*Field {
	if m != nil {
		return m.Fields
	}
	return nil
}

func (m *Point) GetTags() []*Tag {
	if m != nil {
		return m.Tags
	}
	return nil
}

type WriteShardResponse struct {
	Code             *int32  `protobuf:"varint,1,req" json:"Code,omitempty"`
	Message          *string `protobuf:"bytes,2,opt" json:"Message,omitempty"`
	XXX_unrecognized []byte  `json:"-"`
}

func (m *WriteShardResponse) Reset()         { *m = WriteShardResponse{} }
func (m *WriteShardResponse) String() string { return proto.CompactTextString(m) }
func (*WriteShardResponse) ProtoMessage()    {}

func (m *WriteShardResponse) GetCode() int32 {
	if m != nil && m.Code != nil {
		return *m.Code
	}
	return 0
}

func (m *WriteShardResponse) GetMessage() string {
	if m != nil && m.Message != nil {
		return *m.Message
	}
	return ""
}

func init() {
}