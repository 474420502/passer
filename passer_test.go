package passer

import (
	"context"
	"errors"
	"log"
	"reflect"
	"strings"
	"testing"
	"time"
)

type ptype struct {
	Key string
}

type presult struct {
	Key string
}

func TestCase1(t *testing.T) {
	pr := NewPasser[*presult]()
	pr.RegisterPasser(&ptype{}, func(ctx context.Context, obj any) (*presult, error) {
		log.Println(obj)
		v := obj.(*ptype)

		if v.Key != "haha" {
			t.Error(v.Key)
		}

		pt := presult{
			Key: v.Key + "+result",
		}
		return &pt, nil
	})
	data, err := pr.PackToBytes(&ptype{
		Key: "haha",
	})
	if err != nil {
		panic(err)
	}

	r, err := pr.ExecuteWithBytes(context.TODO(), data)
	if err != nil {
		t.Error(err)
	}

	if r.Key != "haha+result" {
		t.Error(r.Key != "haha+result")
	}
}

type testStructA struct {
	FieldA int
	FieldB string
}

type testStructB struct {
	FieldA float64
	FieldB string
}

type testResult struct {
	ResultField string
}

func TestPasser_RegisterPasser(t *testing.T) {
	pr := NewPasser[testResult]()
	if pr == nil {
		t.Fatal("Failed to create a new Passer instance")
	}

	// 正常注册不同类型处理器
	fnA := func(_ context.Context, obj any) (testResult, error) {
		return testResult{ResultField: "from A"}, nil
	}
	fnB := func(_ context.Context, obj any) (testResult, error) {
		return testResult{ResultField: "from B"}, nil
	}
	pr.RegisterPasser(testStructA{}, fnA)
	pr.RegisterPasser(testStructB{}, fnB)

	_, existsA := pr.registedObject[reflect.TypeOf(testStructA{}).String()]
	_, existsB := pr.registedObject[reflect.TypeOf(testStructB{}).String()]
	if !existsA {
		t.Errorf("testStructA should be registered but was not")
	}
	if !existsB {
		t.Errorf("testStructB should be registered but was not")
	}

	// 尝试注册相同类型的不同处理函数
	pr.RegisterPasser(testStructA{}, fnB)
}

func TestPasser_PackToBytes(t *testing.T) {
	pr := NewPasser[testResult]()
	registeredFn := func(_ context.Context, _ any) (testResult, error) {
		return testResult{}, nil
	}
	pr.RegisterPasser(testStructA{}, registeredFn)

	if !pr.HasRegister(testStructA{}) {
		t.Error("pr.HasRegister error")
	}

	if pr.HasRegister(testStructB{}) {
		t.Error("pr.HasRegister error")
	}

	obj := testStructA{FieldA: 1, FieldB: "test"}
	data, err := pr.PackToBytes(obj)
	if err != nil {
		t.Fatalf("PackToBytes failed unexpectedly: %v", err)
	}

	// 验证 ExecuteWithBytes 成功处理合法的 PackToBytes 输出
	_, err = pr.ExecuteWithBytes(context.Background(), data)
	if err != nil {
		t.Fatalf("ExecuteWithBytes failed on valid PackToBytes output: %v", err)
	}

	// 测试未注册类型
	unregisteredObj := testStructB{}
	_, err = pr.PackToBytes(unregisteredObj)
	if err == nil {
		t.Error("Expected error when attempting to pack an unregistered type, got nil")
	} else if !strings.Contains(err.Error(), "is not registed") {
		t.Errorf("Expected error message about unregistered type, got: %s", err.Error())
	}
}

func TestPasser_ExecuteWithBytes(t *testing.T) {
	pr := NewPasser[testResult]()
	executionFn := func(_ context.Context, input any) (testResult, error) {
		in := input.(testStructA)
		return testResult{ResultField: in.FieldB}, nil
	}
	pr.RegisterPasser(testStructA{}, executionFn)

	obj := testStructA{FieldA: 1, FieldB: "execute_test"}
	data, err := pr.PackToBytes(obj)
	if err != nil {
		t.Fatalf("Failed to pack object into bytes: %v", err)
	}

	result, err := pr.ExecuteWithBytes(context.Background(), data)
	if err != nil {
		t.Fatalf("ExecuteWithBytes failed unexpectedly: %v", err)
	}
	if result.ResultField != "execute_test" {
		t.Errorf("Expected ResultField to match the original input's FieldB; got '%s' instead of 'execute_test'", result.ResultField)
	}

	// 测试带有上下文取消的情况
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err = pr.ExecuteWithBytes(cancelCtx, data)
	if err == nil || !strings.Contains(err.Error(), "context canceled") {
		t.Errorf("Expected error due to cancelled context, got: %v", err)
	}

	// 测试错误处理流程
	badData := []byte("invalid serialized data")
	_, err = pr.ExecuteWithBytes(context.Background(), badData)
	if err == nil || err != ErrUnknown {
		t.Errorf("Expected error when executing with invalid serialized data, got: %v", err)
	}
}

func TestPasser_MultipleTypes(t *testing.T) {
	pr := NewPasser[testResult]()
	fnA := func(_ context.Context, _ any) (testResult, error) { return testResult{}, nil }
	fnB := func(_ context.Context, _ any) (testResult, error) { return testResult{}, nil }

	pr.RegisterPasser(testStructA{}, fnA)
	pr.RegisterPasser(testStructB{}, fnB)

	for _, typ := range []struct {
		input any
		fn    Dofunc[testResult]
	}{
		{testStructA{FieldA: 1}, fnA},
		{testStructB{FieldA: 1.5}, fnB},
	} {
		data, err := pr.PackToBytes(typ.input)
		if err != nil {
			t.Fatalf("Failed to pack %T into bytes: %v", typ.input, err)
		}

		_, err = pr.ExecuteWithBytes(context.Background(), data)
		if err != nil {
			t.Errorf("ExecuteWithBytes failed for type %T: %v", typ.input, err)
		}
	}
}

func TestPasser_PackToBytes_ZeroValues(t *testing.T) {
	pr := NewPasser[testResult]()
	// 注册一个支持零值的对象处理器
	pr.RegisterPasser(testStructA{}, func(_ context.Context, _ any) (testResult, error) {
		return testResult{}, nil
	})

	// 测试零值对象序列化
	zeroValueObj := testStructA{}
	data, err := pr.PackToBytes(zeroValueObj)
	if err != nil {
		t.Fatalf("PackToBytes failed unexpectedly for zero value: %v", err)
	}

	// 反序列化验证
	_, err = pr.ExecuteWithBytes(context.Background(), data)
	if err != nil {
		t.Errorf("ExecuteWithBytes failed for zero value: %v", err)
	}
}

func TestPasser_ExecuteWithBytes_ErrorPropagation(t *testing.T) {
	pr := NewPasser[testResult]()
	failingFn := func(_ context.Context, _ any) (testResult, error) {
		return testResult{}, errors.New("forced execution error")
	}
	pr.RegisterPasser(testStructA{}, failingFn)

	obj := testStructA{FieldA: 1, FieldB: "error_test"}
	data, err := pr.PackToBytes(obj)
	if err != nil {
		t.Fatalf("Failed to pack object into bytes for error propagation test: %v", err)
	}

	_, err = pr.ExecuteWithBytes(context.Background(), data)
	if err == nil || !strings.Contains(err.Error(), "forced execution error") {
		t.Errorf("Expected error from failing function call, got: %v", err)
	}
}

func TestPasser_ExecuteWithBytes_ContextTimeout(t *testing.T) {
	pr := NewPasser[testResult]()
	slowExecutionFn := func(ctx context.Context, _ any) (testResult, error) {
		select {
		case <-ctx.Done():
			return testResult{}, ctx.Err()
		case <-time.After(time.Second):
			// 这里模拟一个长时间运行的操作
			return testResult{}, nil
		}
	}
	pr.RegisterPasser(testStructA{}, slowExecutionFn)

	obj := testStructA{FieldA: 1, FieldB: "timeout_test"}
	data, err := pr.PackToBytes(obj)
	if err != nil {
		t.Fatalf("Failed to pack object into bytes for timeout test: %v", err)
	}

	timeoutCtx, cancel := context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	_, err = pr.ExecuteWithBytes(timeoutCtx, data)
	if err == nil || err != ErrTimeout {
		t.Errorf("Expected error due to context timeout, got: %v", err)
	}
}

func TestPasser_ContextCancellationSignal(t *testing.T) {
	pr := NewPasser[testResult]()
	cancellableFn := func(ctx context.Context, _ any) (testResult, error) {
		select {
		case <-ctx.Done():
			return testResult{}, ctx.Err()
		case <-time.After(time.Second):
			return testResult{}, errors.New("operation should have been cancelled")
		}
	}
	pr.RegisterPasser(testStructA{}, cancellableFn)

	obj := testStructA{FieldA: 1}
	data, err := pr.PackToBytes(obj)
	if err != nil {
		t.Fatalf("Failed to pack object: %v", err)
	}

	// 创建一个立即取消的上下文
	immediateCancelCtx, cancel := context.WithCancel(context.Background())
	cancel()

	// 验证函数立即响应取消信号
	_, err = pr.ExecuteWithBytes(immediateCancelCtx, data)
	if err == nil || !strings.Contains(err.Error(), "canceled") {
		t.Errorf("Expected error due to immediate context cancellation, got: %v", err)
	}
}
