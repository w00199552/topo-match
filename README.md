# topo-match

通过逻辑环境拓扑在多张测试床上匹配空闲可用物理环境的工具。

## 功能

- **子树同构匹配**：逻辑拓扑（小树）在测试床（大树）上查找结构同构的空闲子树
- **匹配规则**：device_type 相同、父子结构一致（无序孩子）、property 非空时精确匹配、Link 方向精确匹配
- **并发匹配**：多张测试床并行匹配，使用 Go 协程
- **双模式运行**：CLI 命令行 + REST API 服务

## 安装

```bash
go build -o topo-match ./cmd/topo-match/
```

## 使用

### CLI 模式

```bash
topo-match --logic logic_topo.xml --testbed testbed1.xml,testbed2.xml
```

### 服务模式

```bash
topo-match --serve --port 8080
```

### REST API

```bash
# 添加测试床
curl -X POST http://localhost:8080/api/v1/testbeds \
  -H "Content-Type: application/json" \
  -d '{"name": "testbed1", "file_path": "/path/to/testbed.xml"}'

# 分配环境
curl -X POST http://localhost:8080/api/v1/alloc \
  -H "Content-Type: application/json" \
  -d '{"logic_file_path": "/path/to/logic.xml", "testbed_names": ["testbed1"]}'

# 释放环境
curl -X POST http://localhost:8080/api/v1/free \
  -H "Content-Type: application/json" \
  -d '{"alloc_result": {...}}'
```

## 交叉编译

```bash
make build-linux     # Linux amd64
make build-windows   # Windows amd64
make build-all       # 全平台
```

## 测试

```bash
go test ./... -v
```

## XML 格式

### 逻辑拓扑

```xml
<logic_topology>
    <devices>
        <device>
            <properties>
                <property id="uuid_1"/>
                <property name="objname">fc1</property>
            </properties>
            <devicetype>fc</devicetype>
            <devices>
                <device>...</device>
            </devices>
        </device>
    </devices>
    <links>
        <link source="uuid_3" target="uuid_4" dir="单向"/>
    </links>
</logic_topology>
```

### 测试床

```xml
<testbed>
    <devices>...</devices>
    <links>
        <link source="uuid_3" target="uuid_4" dir="双向"/>
    </links>
</testbed>
```

Link dir 可选值：`单向`、`双向`（默认双向）
