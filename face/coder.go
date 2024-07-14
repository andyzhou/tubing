package face

import (
	"encoding/json"
	"fmt"
	"github.com/andyzhou/tubing/define"
	"google.golang.org/protobuf/proto"
)

/*
 * @author Andy Chow <diudiu8848@163.com>
 * message en/decoder
 */

//face info
type Coder struct {}

//construct
func NewCoder() *Coder {
	this := &Coder{}
	return this
}

//encode message
func (f *Coder) Marshal(contentType int, content proto.Message) ([]byte, error) {
	var (
		data []byte
		err error
	)
	switch contentType {
	case define.MessageTypeOfJson:
		data, err = f.marshalJson(content)
		break
	case define.MessageTypeOfOctet:
		data, err = f.marshalProto(content)
		break
	default:
		err = fmt.Errorf("Decoder:Marshal, unsupported content type:%v", contentType)
		break
	}
	return data, err
}

//decode message
func (f *Coder) Unmarshal(contentType int, content []byte, req proto.Message) error {
	var (
		err error
	)
	switch contentType {
	case define.MessageTypeOfJson:
		err = f.unmarshalJson(content, req)
		break
	case define.MessageTypeOfOctet:
		err = f.unmarshalProto(content, req)
		break
	default:
		err = fmt.Errorf("Coder:Unmarshal, unsupported content type:%v", contentType)
		break
	}
	return err
}

//encode proto octet to proto.Message
func (f *Coder) marshalProto(content proto.Message) ([]byte, error) {
	return proto.Marshal(content)
}

//encode json
func (f *Coder) marshalJson(content proto.Message) ([]byte, error) {
	return json.Marshal(content)
}

//decode proto octet to proto.Message
func (f *Coder) unmarshalProto(data []byte, req proto.Message) error {
	return proto.Unmarshal(data, req)
}

//decode json
func (f *Coder) unmarshalJson(data []byte, req proto.Message) error {
	return json.Unmarshal(data, req)
}