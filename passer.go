// 包 passer 实现了通用的函数执行和序列化功能。
package passer

import (
	"bytes"
	"context"
	"encoding/gob"
	"fmt"
	"reflect"
	"sync"
)

// 初始化 gob，注册一些基本类型，以便于序列化和反序列化。
func init() {
	gob.Register(map[any]any{})
	gob.Register(map[string]interface{}{})
	gob.Register([]interface{}{})
	gob.Register([]map[any]any{})
	gob.Register([]map[string]any{})
}

// executeResult 定义了函数执行的结果，包含一个指向结果的指针和可能的错误信息。
type executeResult[RESULT any] struct {
	Result *RESULT
	Error  error
}

// 分隔符变量 sep 用于在序列化后的字节流中区分类型标识和实际数据。
var sep = []byte{'\x1F', '\x02', '\x1E'}

// Dofunc 定义了一个泛型函数签名，它接收一个上下文和任意类型参数，并返回指定泛型类型的结果和错误。
type Dofunc[RESULT any] func(ctx context.Context, obj any) (RESULT, error)

// passerType 结构体保存了已注册函数的相关信息，包括函数处理的数据类型和该类型对应的执行函数。
type passerType[RESULT any] struct {
	SType  reflect.Type   // 数据类型
	Dofunc Dofunc[RESULT] // 针对该类型数据执行的函数
}

// Passer 结构体用于管理所有已注册函数的信息，通过互斥锁保证线程安全。
type Passer[RESULT any] struct {
	mu             sync.Mutex                     // 用于保护共享资源的互斥锁
	registedObject map[string]*passerType[RESULT] // 注册表，键为类型字符串，值为 passerType
}

// NewPasser 创建并初始化一个新的 Passer 实例。
func NewPasser[RESULT any]() *Passer[RESULT] {
	return &Passer[RESULT]{
		registedObject: make(map[string]*passerType[RESULT]),
	}
}

// RegisterPasser 注册一个函数到 Passer 中，使得在后续执行过程中可以根据数据类型调用对应的函数。 注意不返回已经存在的函数
// 参数：
//
//	t 代表要处理的数据类型实例
//	do 是一个符合 Dofunc 签名的函数，用于处理 ctx 上下文和类型 t 的数据
func (p *Passer[RESULT]) RegisterPasser(t any, do Dofunc[RESULT]) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 获取类型 t 的反射类型
	stype := reflect.TypeOf(t)

	// 将类型和对应处理函数保存到注册表
	p.registedObject[stype.String()] = &passerType[RESULT]{
		SType:  stype,
		Dofunc: do,
	}
}

// HasRegister 坚持注册函数是否存在
func (p *Passer[RESULT]) HasRegister(t any) bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 获取类型 t 的反射类型
	stype := reflect.TypeOf(t)

	_, ok := p.registedObject[stype.String()]

	return ok
}

// PackToBytes 将给定的数据对象序列化成字节数组，其中包括类型标识和序列化后的数据。
// 参数：
//
//	obj 是要序列化的任意数据对象
//
// 返回：
//
//	序列化后的字节数组，以及可能出现的错误
func (p *Passer[RESULT]) PackToBytes(obj any) ([]byte, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	stype := reflect.TypeOf(obj)
	stypestr := stype.String()

	// 检查类型是否已经注册
	if _, ok := p.registedObject[stypestr]; !ok {
		return nil, fmt.Errorf("struct is not registed: %v", obj)
	}

	var buf bytes.Buffer
	buf.Write([]byte(stypestr)) // 写入类型标识
	buf.Write(sep)              // 写入分隔符
	encoder := gob.NewEncoder(&buf)
	err := encoder.Encode(obj) // 序列化数据并写入缓冲区
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), err
}

// ExecuteWithBytes 根据序列化后的字节数组还原数据类型并执行相应的注册函数。
// 参数：
//
//	ctx 是一个上下文对象，可用于取消或超时操作
//	data 是之前由 PackToBytes 生成的序列化字节数组
//
// 返回：
//
//	泛型结果类型 RESULT 的实例，以及执行过程中的错误
func (p *Passer[RESULT]) ExecuteWithBytes(ctx context.Context, data []byte) (RESULT, error) {
	var result RESULT
	var err error

	// 查找并提取数据类型标识
	idx := bytes.Index(data, sep)
	if idx == -1 {
		return result, ErrUnknown
	}

	p.mu.Lock()
	// 根据类型标识查找对应的注册函数
	passer, ok := p.registedObject[string(data[:idx])]
	if !ok {
		p.mu.Unlock()
		return result, ErrUnknown
	}
	p.mu.Unlock()

	// 创建对应类型的零值对象
	obj := reflect.New(passer.SType)

	// 从数据中提取并反序列化实际数据
	decoder := gob.NewDecoder(bytes.NewBuffer(data[idx+len(sep):]))
	err = decoder.DecodeValue(obj)
	if err != nil {
		return result, err
	}

	// 使用 goroutine 异步执行函数，并通过 channel 接收执行结果
	done := make(chan executeResult[RESULT])

	go func() {
		defer close(done)
		resultVal, execErr := passer.Dofunc(ctx, obj.Elem().Interface())
		done <- executeResult[RESULT]{&resultVal, execErr}
	}()

	// 监听上下文的 Done 信号和执行结果 channel
	select {
	case <-ctx.Done():
		// 超时或取消时返回错误信息
		return result, ctx.Err()
	case rdone := <-done:
		if rdone.Error != nil {
			return result, rdone.Error
		}
		return *rdone.Result, nil
	}
}
