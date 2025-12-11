# gonfig - Go 配置管理库

gonfig 是一个功能强大的 Go 语言配置管理库，支持多种配置源和格式，提供了统一的配置加载和监听机制。

## 特性

- **多源支持**：支持从环境变量、文件、Consul、Nacos 等多种来源加载配置
- **多格式支持**：内置支持 JSON、YAML、TOML、ENV 等常见配置格式
- **热更新**：支持配置变更监听和自动重新加载
- **Protobuf 集成**：与 Protobuf 深度集成，可自动生成配置管理代码
- **类型安全**：基于结构化数据和强类型配置对象
- **易于扩展**：提供清晰的接口用于添加新的配置源和格式

## 安装

```bash
go get github.com/go-leo/gonfig
```

## 快速开始

### 1. 定义配置结构

首先使用 Protobuf 定义配置结构，并启用 gonfig：

```protobuf
syntax = "proto3";
package example;
option go_package = "github.com/go-leo/gonfig/example/configs;configs";

import "leo/gonfig/annotations.proto";

message AppConfig {
  option (leo.gonfig.enable) = true;
  
  string addr = 1;
  int32 port = 2;
  string environment = 3;
}
```

运行以下命令生成代码：

```bash
protoc --go_out=. --gonfig_out=. configs/*.proto
```

### 2. 使用环境变量配置

```go
package main

import (
    "context"
    "fmt"
    "os"
    
    "github.com/go-leo/gonfig/example/configs"
    "github.com/go-leo/gonfig/resource/env"
)

func main() {
    // 设置环境变量
    os.Setenv("APP_ADDR", "localhost")
    os.Setenv("APP_PORT", "8080")
    os.Setenv("APP_ENVIRONMENT", "development")
    
    // 创建环境变量资源配置
    envResource, err := env.New("APP_", time.Second)
    if err != nil {
        panic(err)
    }
    
    // 加载配置
    if err := configs.LoadAppConfigConfig(context.TODO(), envResource); err != nil {
        panic(err)
    }
    
    // 获取配置
    config := configs.GetAppConfigConfig()
    fmt.Printf("Address: %s\n", config.GetAddr())
    fmt.Printf("Port: %d\n", config.GetPort())
    fmt.Printf("Environment: %s\n", config.GetEnvironment())
}
```

### 3. 监听配置变化

```go
errFunc := func(err error) {
    fmt.Printf("配置监听错误: %v\n", err)
}

// 开始监听配置变化
stop, err := configs.WatchAppConfigConfig(context.TODO(), envResource, errFunc)
if err != nil {
    panic(err)
}
defer stop(context.TODO())
```

## 支持的配置源

### 1. 环境变量 (env)

```go
import "github.com/go-leo/gonfig/resource/env"

resource, err := env.New("PREFIX_", time.Second)
```

### 2. 文件 (file)

```go
import "github.com/go-leo/gonfig/resource/file"

resource, err := file.New("/path/to/config.json")
```

### 3. Consul

```go
import "github.com/go-leo/gonfig/resource/consul"

client, _ := api.NewClient(api.DefaultConfig())
resource, err := consul.New(client, "config/key")
```

### 4. Nacos

```go
import "github.com/go-leo/gonfig/resource/nacos"

resource, err := nacos.New(client, "group", "dataId")
```

## 支持的配置格式

- **JSON**: `.json` 文件扩展名
- **YAML**: `.yaml` 或 `.yml` 文件扩展名
- **TOML**: `.toml` 文件扩展名
- **ENV**: 环境变量格式（键值对）

## Protobuf 注解

使用 `leo.gonfig.enable = true` 注解启用配置生成功能：

```protobuf
message Config {
  option (leo.gonfig.enable) = true;
  
  string field = 1;
}
```

这会生成以下辅助函数：

- [GetConfig()](file:///home/soyacen/Workspace/github.com/go-leo/gonfig/cmd/protoc-gen-gonfig/gen/generator.go#L87-L89) - 获取当前配置实例
- [LoadConfig()](file:///home/soyacen/Workspace/github.com/go-leo/gonfig/cmd/protoc-gen-gonfig/gen/generator.go#L95-L97) - 从资源加载配置
- [WatchConfig()](file:///home/soyacen/Workspace/github.com/go-leo/gonfig/cmd/protoc-gen-gonfig/gen/generator.go#L99-L101) - 监听配置变化

## 许可证

MIT License

## 贡献

欢迎提交 Issue 和 Pull Request 来改进这个项目。