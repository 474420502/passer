// passer包实现通用的函数执行和序列化
package passer

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"reflect"
	"sync"
)

func init() {
	gob.Register(map[string]interface{}{})
}

// 执行结果
type executeResult[RESULT any] struct {
	result *RESULT
	err    error
}

// 分隔符
var sep = []byte("!?@#")

// 注册函数类型
type Dofunc[RESULT any] func(ctx context.Context, obj any) (RESULT, error)

// 已注册函数信息
type passerType[RESULT any] struct {
	SType  reflect.Type
	Dofunc Dofunc[RESULT]
}

// Passer 管理注册信息
type Passer[RESULT any] struct {
	// 锁保护共享数据
	mu sync.Mutex

	// 注册表
	registedObject map[string]*passerType[RESULT]
}

// 创建
func NewPasser[RESULT any]() *Passer[RESULT] {
	return &Passer[RESULT]{
		registedObject: make(map[string]*passerType[RESULT]),
	}
}

// 注册函数
func (p *Passer[RESULT]) RegisterPasser(t any, do Dofunc[RESULT]) {

	// 加锁
	p.mu.Lock()
	defer p.mu.Unlock()

	// 获取类型
	stype := reflect.TypeOf(t)

	// 保存注册信息
	p.registedObject[stype.String()] = &passerType[RESULT]{
		SType:  stype,
		Dofunc: do,
	}
}

// 序列化
func (p *Passer[RESULT]) PackToBytes(obj any) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	stype := reflect.TypeOf(obj)
	stypestr := stype.String()
	// 检查注册
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

// 执行
func (p *Passer[RESULT]) ExecuteWithBytes(ctx context.Context, data []byte) (RESULT, error) {
	var result RESULT
	var err error

	// 找传递数据的类型
	idx := bytes.Index(data, sep)

	p.mu.Lock()
	// 找到对应类型的注册函数
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
