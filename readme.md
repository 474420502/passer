# Passer

Passer是一个Go语言编写的通用函数执行和序列化组件。

## 简介

Passer可以注册和执行各种函数,并且支持类型安全的序列化与反序列化。主要功能包括:

- 支持函数注册与执行
- 通用的类型安全序列化/反序列化
- 并发安全设计
- 结果超时控制

## 用法

### 1. 创建Passer

```go
p := passer.NewPasser[int]()
```

### 2. 注册函数

```go
func add(cxt context.Context, args AddArgs) (int, error) {
  // ...
}

p.Register(AddArgs{}, add) 
```

### 3. 执行函数

```go
data, _ := p.PackToBytes(AddArgs{A: 1, B: 2}) 

result, err := p.ExecuteWithBytes(context.Background(), data)
```

### 4. 超时控制

```go
ctx, cancel := context.WithTimeout(context.Background(), time.Second)
defer cancel()

result, err := p.ExecuteWithBytes(ctx, data) 
```

## 开源许可

Passer 使用MIT开源许可证,欢迎大家使用和贡献!

## 贡献指南

- Fork 代码并提交PR
- 提交issue反馈问题
- 代码Review

## 联系作者

email: 474420502@qq.com

欢迎讨论Passer的设计与实现!