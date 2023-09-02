package json

import (
	"encoding/json"
	"errors"
	"strconv"
)

/*
 * base json face
 */

type BaseJson struct {
}

//construct
func NewBaseJson() *BaseJson {
	this := &BaseJson{}
	return this
}

func (j *BaseJson) Str2BigInt(val string) int64 {
	valInt, _ := strconv.ParseInt(val, 10, 64)
	return valInt
}
func (j *BaseJson) Str2Int(val string) int {
	valInt, _ := strconv.Atoi(val)
	return valInt
}

//decode self
func (j *BaseJson) DecodeSelf(data []byte) error {
	//try decode json data
	err := json.Unmarshal(data, j)
	return err
}

//encode json data
func (j *BaseJson) Encode(i interface{}) ([]byte, error) {
	if i == nil {
		return nil, errors.New("invalid parameter")
	}
	//encode json
	resp, err := json.Marshal(i)
	return resp, err
}

func (j *BaseJson) Encode2Str(i interface{}) (string, error) {
	//encode json
	jsonByte, err := j.Encode(i)
	if err != nil {
		return "", err
	}
	return string(jsonByte), nil
}

//decode json data
func (j *BaseJson) Decode(data []byte, i interface{}) error {
	if len(data) <= 0 {
		return errors.New("json data is empty")
	}
	//try decode json data
	err := json.Unmarshal(data, i)
	return err
}

//encode simple kv data
func (j *BaseJson) EncodeSimple(data map[string]interface{}) ([]byte, error) {
	if data == nil {
		return nil, errors.New("json data is empty")
	}
	//try encode json data
	byte, err := json.Marshal(data)
	return byte, err
}

//decode simple kv data
func (j *BaseJson) DecodeSimple(data []byte, kv map[string]interface{}) error {
	if len(data) <= 0 {
		return errors.New("json data is empty")
	}
	//try decode json data
	err := json.Unmarshal(data, &kv)
	return err
}
