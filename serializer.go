package passer

import (
	"bytes"
	"encoding/gob"
	"encoding/json"
)

// Serializer接口
type Serializer interface {
	Encode(v interface{}) ([]byte, error)
	Decode(data []byte, v interface{}) error
}

// JSON序列化实现
type JsonSerializer struct{}

func (j JsonSerializer) Encode(v interface{}) ([]byte, error) {
	return json.Marshal(v)
}

func (j JsonSerializer) Decode(data []byte, v interface{}) error {
	return json.Unmarshal(data, v)
}

// gob序列化
type GobSerializer struct{}

func (g GobSerializer) Encode(v interface{}) ([]byte, error) {
	var buf bytes.Buffer
	enc := gob.NewEncoder(&buf)
	err := enc.Encode(v)
	if err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

func (g GobSerializer) Decode(data []byte, v interface{}) error {

	dec := gob.NewDecoder(bytes.NewBuffer(data))
	err := dec.Decode(v)
	if err != nil {
		return err
	}

	return nil
}
