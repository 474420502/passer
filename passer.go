package passer

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"reflect"
	"sync"
)

type executeResult[RESULT any] struct {
	result *RESULT
	err    error
}

var sep = []byte("!?@#")

type Dofunc[RESULT any] func(cxt context.Context, obj any) (RESULT, error)

type passerType[RESULT any] struct {
	SType  reflect.Type
	Dofunc Dofunc[RESULT]
}

type Passer[RESULT any] struct {
	mu             sync.Mutex
	registedObject map[string]*passerType[RESULT]
}

func NewPasser[RESULT any]() *Passer[RESULT] {
	return &Passer[RESULT]{
		registedObject: make(map[string]*passerType[RESULT]),
	}
}

func (p *Passer[RESULT]) RegisterPasser(t any, do Dofunc[RESULT]) {
	p.mu.Lock()
	defer p.mu.Unlock()
	stype := reflect.TypeOf(t)

	p.registedObject[stype.String()] = &passerType[RESULT]{
		SType:  stype,
		Dofunc: do,
	}
}

func (p *Passer[RESULT]) PackToBytes(obj any) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	stype := reflect.TypeOf(obj)
	stypestr := stype.String()
	if _, ok := p.registedObject[stypestr]; !ok {
		return nil, fmt.Errorf("struct is not registed: %v", obj)
	}

	var buf bytes.Buffer
	buf.Write([]byte(stypestr))
	buf.Write(sep)
	err := gob.NewEncoder(&buf).Encode(obj)
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), err
}

func (p *Passer[RESULT]) ExecuteWithBytes(ctx context.Context, data []byte) (RESULT, error) {
	var result RESULT
	var err error

	idx := bytes.Index(data, sep)

	p.mu.Lock()
	passer, ok := p.registedObject[string(data[:idx])]
	if !ok {
		p.mu.Unlock()
		return result, nil
	}
	p.mu.Unlock()

	obj := reflect.New(passer.SType)
	var buf = bytes.NewBuffer(data[idx+4:])
	err = gob.NewDecoder(buf).DecodeValue(obj)
	if err != nil {
		return result, err
	}

	done := make(chan executeResult[RESULT])
	defer close(done)

	go func() {
		result, err = passer.Dofunc(ctx, obj.Elem().Interface())
		done <- executeResult[RESULT]{&result, err}
	}()

	select {
	case <-ctx.Done():
		// 处理超时错误
		return result, fmt.Errorf("%v execute timeout", obj)
	case rdone := <-done:
		if rdone.err != nil {
			return result, rdone.err
		}
		return *rdone.result, nil
	}
}
