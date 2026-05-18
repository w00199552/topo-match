# topo-match

> **版本**: v0.2.0

Go 语言算法服务 / CLI 工具 —— 通过逻辑拓扑在多张测试床上匹配空闲物理环境。

---

## 目录

1. [项目概述](#1-项目概述)
   - [1.1 问题定义](#11-问题定义)
   - [1.2 一图看懂：逻辑拓扑在多测试床上的匹配过程](#12-一图看懂逻辑拓扑在多测试床上的匹配过程)
   - [1.3 核心能力](#13-核心能力)
   - [1.4 适用规模](#14-适用规模)
2. [系统架构](#2-系统架构)
3. [数据模型](#3-数据模型)
4. [核心算法](#4-核心算法)
5. [约束与限制](#5-约束与限制)
6. [使用方式](#6-使用方式)
7. [快速上手案例](#7-快速上手案例)
8. [API 参考](#8-api-参考)
9. [扩展路线](#9-扩展路线)

---

## 1. 项目概述

### 1.1 问题定义

在测试床（Testbed）管理场景中，存在大量物理设备组成的拓扑树。当测试任务需要一个特定的逻辑拓扑环境时，需要从多张测试床中找到**空闲的、结构匹配的**物理子树，并将其分配给任务使用。

核心问题可以形式化为：**无序子树同构匹配** —— 在一棵大树（测试床）上找到一棵小树（逻辑拓扑）的同构子树，且该子树所有节点当前处于空闲状态。

### 1.2 一图看懂：逻辑拓扑在多测试床上的匹配过程

> 下方图示展示了完整的匹配流程：一张逻辑拓扑同时搜索两张测试床，结构 + 链路方向都匹配才命中。

```
╔══════════════════════════════════════════════════════════════════════════════════╗
║                          我需要这样的测试环境：                                   ║
║                         （逻辑拓扑 Logic Topology）                               ║
║                                                                                ║
║                          fc1 (fc)                                              ║
║                           │                                                    ║
║                     cluster1 (cluster)                                         ║
║                       ╱        ╲                                               ║
║                  cna1 (cna)   cna2 (cna)                                       ║
║                       ────▶─────                                               ║
║                      单向 Link                                                  ║
╚══════════════════════════════════════════════════════════════════════════════════╝
                                      │
                    ┌─────────────────┴──────────────────┐
                    │         并发搜索两张测试床            │
                    ▼                                     ▼
╔═══════════════════════════════╗     ╔═══════════════════════════════╗
║   测试床 A (Testbed A)        ║     ║   测试床 B (Testbed B)        ║
║                               ║     ║                               ║
║   fc10 (fc)  [idle]           ║     ║   fc30 (fc)  [idle]           ║
║     │                         ║     ║     │                         ║
║   cluster10 (cluster) [idle]  ║     ║   cluster30 (cluster) [idle]  ║
║     ╱        ╲                ║     ║     ╱        ╲                ║
║  cna10     cna20              ║     ║  cna30     cna31              ║
║  (cna)     (cna)              ║     ║  (cna)     (cna)              ║
║  [idle]    [idle]             ║     ║  [idle]    [idle]             ║
║     ◀──────▶                  ║     ║     ──────▶                   ║
║     双向 Link                  ║     ║     单向 Link                  ║
║                               ║     ║                               ║
║   ┌─────────────────────┐     ║     ║                               ║
║   │ ✅ 结构匹配          │     ║     ║   ┌─────────────────────┐     ║
║   │ ❌ 链路方向不匹配     │     ║     ║   │ ✅ 结构匹配          │     ║
║   │   逻辑要单向          │     ║     ║   │ ✅ 链路方向匹配      │     ║
║   │   物理是双向          │     ║     ║   │   逻辑要单向 ✅       │     ║
║   │   单向 ≠ 双向         │     ║     ║   │   物理也是单向 ✅     │     ║
║   └─────────────────────┘     ║     ║   └─────────────────────┘     ║
║                               ║     ║                               ║
║        ❌ 匹配失败             ║     ║        ✅ 匹配成功             ║
╚═══════════════════════════════╝     ╚═══════════════════════════════╝
                                                    │
                                                    ▼
╔══════════════════════════════════════════════════════════════════════════════════╗
║                            分配结果 AllocResult                                  ║
║                                                                                ║
║   逻辑节点         物理节点 (测试床 B)         状态变更                           ║
║   ─────────       ──────────────────         ────────                           ║
║   fc1        ──►   fc30                   idle → used                          ║
║   cluster1   ──►   cluster30              idle → used                          ║
║   cna1       ──►   cna30                  idle → used                          ║
║   cna2       ──►   cna31                  idle → used                          ║
║                                                                                ║
║   Link: cna1 ──▶ cna2  ──►  cna30 ──▶ cna31  ✅ 方向一致                       ║
╚══════════════════════════════════════════════════════════════════════════════════╝
```

**图解要点**：

| # | 规则 | 图中体现 |
|---|------|----------|
| 1 | device_type 必须一致 | fc→fc, cluster→cluster, cna→cna，类型逐层对齐 |
| 2 | 孩子数量必须一致 | cluster 下都是 2 个 cna 孩子 |
| 3 | 孩子无序匹配 | cna1 可以匹配 cna30 或 cna31，顺序不重要 |
| 4 | Link 方向精确匹配 | 逻辑要单向 → 测试床 A 双向❌ / 测试床 B 单向✅ |
| 5 | 空闲节点才可分配 | 所有节点 [idle] 才能被选中 |
| 6 | 多测试床并发 | 同时搜索 A 和 B，返回首个匹配 |

---

下面再看一个**共享分配**的场景——同一张测试床被两个任务复用：

```
╔══════════════════════════════════════════════════════════════════════════╗
║  测试床 C：fc40 分支的节点标记了 Share=true，允许共享                      ║
║                                                                        ║
║   fc40 (fc)  [idle]  Share=true                                        ║
║     │                                                                  ║
║   cluster40 (cluster) [idle]  Share=true                               ║
║     ╱        ╲                                                         ║
║  cna40      cna41                                                      ║
║  (cna)      (cna)                                                      ║
║  [idle]     [idle]                                                     ║
║  Share=true Share=true                                                 ║
║     ──────▶                                                            ║
║     单向 Link                                                           ║
╚══════════════════════════════════════════════════════════════════════════╝

  任务 1 Alloc                           任务 2 Alloc
  ─────────────                          ─────────────
  fc1 ──► fc40                           fc1 ──► fc40
  cluster1 ──► cluster40                 cluster1 ──► cluster40
  cna1 ──► cna40                         cna1 ──► cna40
  cna2 ──► cna41                         cna2 ──► cna41

  分配后状态：                            分配后状态：
  fc40     [used] RefCount=1             fc40     [used] RefCount=2
  cluster40 [used] RefCount=1            cluster40 [used] RefCount=2
  cna40    [used] RefCount=1             cna40    [used] RefCount=2
  cna41    [used] RefCount=1             cna41    [used] RefCount=2

  ─────────────────────────────────────────────────────────────
  任务 1 Free → RefCount 2→1，节点仍 used（还有任务在用）
  任务 2 Free → RefCount 1→0，节点恢复 idle（无人使用了）
```

### 1.3 核心能力

| 能力 | 说明 |
|------|------|
| **拓扑匹配** | 在测试床物理拓扑中查找与逻辑拓扑结构一致的空闲子树 |
| **多测试床并发** | 同时在多张测试床上并发搜索，返回首个匹配结果 |
| **属性约束** | 支持按 device_type、property 精确匹配过滤 |
| **链路方向校验** | 支持 单向/双向 Link 的方向精确匹配 |
| **共享分配** | Share 节点支持引用计数，允许多次分配 |
| **分配/释放** | Alloc 标记占用，Free 释放回收，完整生命周期管理 |
| **双模式运行** | CLI 一次性执行 + REST API 长驻服务 |

### 1.4 适用规模

| 维度 | 范围 |
|------|------|
| 逻辑拓扑节点数 | 1 ~ 15 |
| 单张测试床节点数 | 1 ~ 30 |
| 测试床数量 | 20 ~ 200 |

---

## 2. 系统架构

### 2.1 整体架构图

```
┌─────────────────────────────────────────────────────┐
│                    用户接入层                         │
│  ┌──────────────┐              ┌──────────────────┐  │
│  │   CLI 模式    │              │   REST API 模式   │  │
│  │  (flag 参数)  │              │  (Fiber HTTP)    │  │
│  └──────┬───────┘              └────────┬─────────┘  │
│         │                               │            │
├─────────┴───────────────────────────────┴────────────┤
│                    业务编排层                         │
│  ┌─────────────────────────────────────────────────┐ │
│  │              Allocator (分配器)                   │ │
│  │  · AddTestbed / RemoveTestbed                    │ │
│  │  · Alloc (并发匹配 + 标记占用)                    │ │
│  │  · Free  (引用计数递减 + 释放)                    │ │
│  │  · GetTestbedStatus / ListTestbeds               │ │
│  └────────────────────┬────────────────────────────┘ │
│                       │                              │
├───────────────────────┴──────────────────────────────┤
│                    算法引擎层                         │
│  ┌─────────────────────────────────────────────────┐ │
│  │              Matcher (匹配器)                     │ │
│  │  · 单根匹配 / 多根回溯匹配                        │ │
│  │  · 无序孩子回溯 (backtrackChildren)               │ │
│  │  · Link 方向校验 (checkLinks)                     │ │
│  └────────────────────┬────────────────────────────┘ │
│                       │                              │
├───────────────────────┴──────────────────────────────┤
│                    数据模型层                         │
│  ┌──────────────────┐  ┌───────────────────────────┐ │
│  │  model.Topology   │  │  model.Node / model.Link  │ │
│  │  · RootDevices    │  │  · UUID / DeviceType      │ │
│  │  · Links          │  │  · Properties / Status    │ │
│  └──────────────────┘  │  · Share / RefCount        │ │
│                         └───────────────────────────┘ │
├──────────────────────────────────────────────────────┤
│                    数据接入层                         │
│  ┌─────────────────────────────────────────────────┐ │
│  │           XML Parser (model.ParseXML)            │ │
│  │  · logic_topology 格式                           │ │
│  │  · testbed 格式                                  │ │
│  │  · 兼容 deivces 拼写错误                          │ │
│  └─────────────────────────────────────────────────┘ │
└─────────────────────────────────────────────────────┘
```

### 2.2 目录结构

```
topo-match/
├── cmd/topo-match/
│   └── main.go              # 入口：CLI / Server 模式分发
├── internal/
│   ├── model/
│   │   ├── node.go          # Node / Link / Topology 数据结构
│   │   └── xml.go           # XML 解析 (logic_topology + testbed)
│   ├── matcher/
│   │   ├── matcher.go       # 子树同构匹配算法
│   │   └── matcher_test.go  # 匹配器单元测试
│   ├── allocator/
│   │   ├── allocator.go     # 分配/释放/引用计数管理
│   │   └── allocator_test.go# 分配器单元测试
│   └── server/
│       └── server.go        # REST API (Fiber)
├── examples/
│   ├── logic_topo.xml       # 示例逻辑拓扑
│   └── testbed.xml          # 示例测试床
├── Makefile                 # 构建/测试/运行快捷命令
├── go.mod
└── go.sum
```

### 2.3 模块依赖关系

```
main.go
  ├── model      (数据结构 + XML 解析)
  ├── matcher    (依赖 model)
  ├── allocator  (依赖 matcher + model)
  └── server     (依赖 allocator + model)
```

依赖方向严格单向，无循环依赖。

---

## 3. 数据模型

### 3.1 核心结构

#### Node — 设备节点

```go
type Node struct {
    UUID       string            // 节点唯一标识，用于 Link 引用
    ObjName    string            // 节点对象名称
    DeviceType string            // 设备类型（必填，匹配第一道关卡）
    Children   []*Node           // 子节点列表（无序）
    Properties map[string]string // 动态属性键值对
    Status     string            // "idle" | "used"（仅测试床节点有效）
    Share      bool              // 是否允许共享分配
    RefCount   int               // 引用计数（内部使用，不序列化）
}
```

#### Link — 节点间链路

```go
type Link struct {
    SourceUUID string  // 源节点 UUID
    TargetUUID string  // 目标节点 UUID
    Dir        LinkDir // "单向" | "双向"（默认 "双向"）
}
```

#### Topology — 完整拓扑

```go
type Topology struct {
    RootDevices []*Node // 根设备列表（支持多根）
    Links       []*Link // 节点间链路
}
```

### 3.2 状态机

节点状态流转：

```
         Alloc 成功              Free (RefCount→0)
  idle ──────────────► used ──────────────────► idle
                           │
                           │ Free (RefCount>0)
                           ▼
                         used  (Share 节点，引用计数递减但仍 >0)
```

**关键规则**：
- 非 Share 节点：Alloc 后 Status 变 `used`，Free 后直接变 `idle`
- Share 节点：每次 Alloc 引用计数 +1，每次 Free 引用计数 -1，仅当 RefCount 归零才恢复 `idle`

### 3.3 XML 格式

#### 逻辑拓扑 (logic_topology)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<logic_topology>
    <devices>
        <device>
            <properties>
                <property id="uuid_1"/>                    <!-- UUID -->
                <property name="objname">fc1</property>    <!-- 对象名 -->
                <property name="version">2</property>      <!-- 自定义属性 -->
            </properties>
            <devicetype>fc</devicetype>
            <devices>                                      <!-- 子设备 -->
                <device>
                    <properties>
                        <property id="uuid_2"/>
                        <property name="objname">cluster1</property>
                    </properties>
                    <devicetype>cluster</devicetype>
                </device>
            </devices>
        </device>
    </devices>
    <links>
        <link source="uuid_1" target="uuid_2" dir="单向"/>
    </links>
</logic_topology>
```

#### 测试床 (testbed)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<testbed>
    <devices>
        <!-- 结构同 logic_topology，可包含多个根 device -->
    </devices>
    <links>
        <link source="uuid_3" target="uuid_4" dir="双向"/>
    </links>
</testbed>
```

**XML 解析兼容性**：
- 根标签自动识别 `<logic_topology>` 或 `<testbed>`
- 子设备标签兼容 `<devices>` 和 `<deivces>`（常见拼写错误）
- Link 的 `dir` 属性缺省时默认为 `"双向"`

---

## 4. 核心算法

### 4.1 问题形式化

给定：
- 逻辑拓扑 `L`（小树，1~15 节点）
- 测试床拓扑 `T`（大树，1~30 节点）

求：映射 `φ: L.nodes → T.nodes`，满足：

1. **类型一致**：`∀n ∈ L, device_type(n) = device_type(φ(n))`
2. **结构同构**：`n` 是 `m` 的孩子 ⟺ `φ(n)` 是 `φ(m)` 的孩子（孩子集合无序）
3. **属性匹配**：`∀n ∈ L, ∀k: n.properties[k] ≠ "" ⟹ n.properties[k] = φ(n).properties[k]`
4. **空闲约束**：`∀n ∈ L, φ(n).status = "idle"` 或 `(φ(n).status = "used" ∧ φ(n).share = true)`
5. **链路一致**：`∀link ∈ L.links, ∃physLink ∈ T.links` 使得方向匹配

### 4.2 匹配算法流程

```
Match(logic, testbed, testbedName)
│
├── 单根? ──► matchSingleRoot
│              │
│              ├── 遍历 testbed 所有节点作为候选根
│              │   ├── matchNode(逻辑根, 候选节点)
│              │   │   ├── 检查 device_type
│              │   │   ├── 检查 status (idle / share+used)
│              │   │   ├── 检查 used 集合 (防重复映射)
│              │   │   ├── 检查 properties (非空精确匹配)
│              │   │   ├── 检查孩子数量一致
│              │   │   └── matchChildren (无序孩子回溯)
│              │   │       └── backtrackChildren
│              │   │           ├── 对每个逻辑孩子，尝试所有物理孩子
│              │   │           ├── 递归 + 回溯 (保存/恢复 mapping 和 used)
│              │   │           └── 全部匹配成功 → true
│              │   └── checkLinks (链路方向校验)
│              └── 返回 MatchResult 或 nil
│
└── 多根? ──► matchMultipleRoots
               │
               └── backtrackRoots
                   ├── 对每个逻辑根设备，尝试所有物理候选
                   ├── 递归 + 回溯 (保存/恢复全局 mapping 和 used)
                   └── 所有根都匹配成功 → checkLinks → 返回结果
```

### 4.3 关键算法详解

#### 4.3.1 无序孩子匹配 (backtrackChildren)

这是算法的核心难点。逻辑拓扑的孩子是**无序的**，即逻辑孩子 `[A, B]` 可以匹配物理孩子 `[B', A']`。

```
backtrackChildren(logicChildren, physChildren, idx, usedPhys, mapping, used):
    if idx == len(logicChildren):
        return true  // 所有逻辑孩子都匹配完毕

    for j = 0 to len(physChildren)-1:
        if usedPhys[j]: continue  // 跳过已使用的物理孩子

        // 保存现场
        savedMapping = copy(mapping)
        savedUsed = copy(used)

        if matchNode(logicChildren[idx], physChildren[j], mapping, used):
            usedPhys[j] = true
            if backtrackChildren(logicChildren, physChildren, idx+1, ...):
                return true
            // 回溯
            usedPhys[j] = false
            restore(mapping, savedMapping)
            restore(used, savedUsed)

    return false  // 当前逻辑孩子无法匹配任何物理孩子
```

**复杂度**：最坏情况 O(k!)，其中 k 为孩子数量。实际场景中 k ≤ 5，可接受。

#### 4.3.2 链路方向校验 (checkLinks)

匹配成功后，还需验证逻辑拓扑中的 Link 在物理拓扑中是否存在方向一致的对应 Link。

```
checkLinks(logic, testbed, mapping):
    for each logicLink in logic.Links:
        physSource = mapping[logicLink.SourceUUID]
        physTarget = mapping[logicLink.TargetUUID]

        found = false
        for each physLink in testbed.Links:
            // 正向匹配：方向一致
            if physLink.Source == physSource && physLink.Target == physTarget:
                if physLink.Dir == logicLink.Dir:
                    found = true; break

            // 双向链路反向匹配
            if logicLink.Dir == "双向" && physLink.Dir == "双向":
                if physLink.Source == physTarget && physLink.Target == physSource:
                    found = true; break

        if !found: return false
    return true
```

**方向匹配规则**：

| 逻辑 Link 方向 | 物理 Link 方向 | 匹配结果 |
|:-:|:-:|:-:|
| 单向 A→B | 单向 A→B | ✅ |
| 单向 A→B | 单向 B→A | ❌ |
| 单向 A→B | 双向 A↔B | ❌ |
| 双向 A↔B | 双向 A↔B | ✅ |
| 双向 A↔B | 双向 B↔A | ✅ |
| 双向 A↔B | 单向 A→B | ❌ |

> **设计决策**：单向不匹配双向，双向不匹配单向。方向必须精确一致，双向允许正反序。

#### 4.3.3 并发匹配 (Allocator.Alloc)

```
Alloc(logic, testbedNames):
    加锁 (mutex)

    并发启动 goroutine:
        for each testbed:
            go matcher.Match(logic, testbed, name) → channel

    收集结果:
        取第一个非 nil 的匹配结果

    if 匹配成功:
        标记物理节点 status="used", RefCount++
        返回 AllocResult
    else:
        返回错误 "无空闲的测试床物理环境可用"

    解锁
```

**并发安全**：当前使用全局 Mutex 保证 Alloc + Free 的串行化。匹配阶段虽然是并发的，但标记占用阶段在锁内完成，避免竞态。

---

## 5. 约束与限制

### 5.1 匹配约束

| 约束 | 说明 |
|------|------|
| **device_type 必须精确匹配** | 逻辑节点和物理节点的 device_type 必须完全一致 |
| **孩子数量必须一致** | 逻辑节点的孩子数必须等于物理候选节点的孩子数（严格结构同构） |
| **property 非空时精确匹配** | 逻辑属性值为空字符串时不约束，非空时必须与物理属性值完全一致 |
| **Link 方向精确匹配** | 单向/双向不互通，双向允许正反序 |
| **used 节点不可分配** | 除非标记 Share=true，否则 status=used 的节点跳过 |
| **同一物理节点不可重复映射** | 同一次匹配中，一个物理 UUID 只能映射一个逻辑 UUID |

### 5.2 系统限制

| 限制 | 当前状态 | 说明 |
|------|----------|------|
| **并发模型** | 全局 Mutex | Alloc 和 Free 串行执行，高并发下可能成为瓶颈 |
| **状态持久化** | 无 | 进程重启后状态丢失，需重新加载测试床 XML |
| **匹配策略** | 首个匹配 | 多测试床并发搜索，返回第一个匹配结果，不做最优选择 |
| **XML 输入** | 仅文件路径 | 不支持直接传入 XML 字符串（CLI 模式） |
| **优雅关闭** | 未实现 | Server 模式下 Ctrl+C 直接退出 |
| **路径安全** | 默认 `/` | Server 模式 dataDir 默认允许所有路径，生产环境需限制 |

### 5.3 已知边界情况

1. **空逻辑拓扑**：RootDevices 为空时直接返回 nil（无匹配）
2. **空属性值**：逻辑属性 `""` 不参与匹配，物理节点有无该属性均可
3. **多根设备**：所有逻辑根设备必须在同一张测试床上匹配成功（跨测试床不行）
4. **Share 节点重复映射**：同一次匹配中，Share 节点也不会被映射两次（used 集合保护）
5. **Link 引用缺失**：如果逻辑 Link 的 Source/Target UUID 不在 mapping 中，匹配失败

---

## 6. 使用方式

### 6.1 构建

```bash
# 本地构建
make build

# 交叉编译
make build-linux      # → bin/topo-match-linux-amd64
make build-windows    # → bin/topo-match-windows-amd64.exe
make build-all        # 同时构建两个平台

# 运行测试
make test
```

### 6.2 CLI 模式

#### 分配环境

```bash
topo-match --logic <逻辑拓扑XML> --testbed <测试床XML1,测试床XML2,...>
```

#### 释放环境

```bash
topo-match --free <分配结果JSON> --testbed <测试床XML1,测试床XML2,...>
```

#### 查看版本

```bash
topo-match --version
```

### 6.3 Server 模式

```bash
topo-match --serve [--port 8080]
```

启动后提供 REST API，详见 [第 8 节](#8-api-参考)。

---

## 7. 快速上手案例

### 7.1 场景描述

**逻辑需求**：需要一个 `fc → cluster → [cna, cna]` 的三层拓扑，两个 cna 之间有单向链路。

**测试床资源**：一张测试床包含两组 fc 拓扑：
- fc10 分支：cna10 ↔ cna20（双向链路）
- fc20 分支：cna30 → cna40（单向链路）

### 7.2 准备 XML 文件

**logic_topo.xml**（逻辑拓扑）：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<logic_topology>
    <devices>
        <device>
            <properties>
                <property id="uuid_1"/>
                <property name="objname">fc1</property>
            </properties>
            <devicetype>fc</devicetype>
            <devices>
                <device>
                    <properties>
                        <property id="uuid_2"/>
                        <property name="objname">cluster1</property>
                    </properties>
                    <devicetype>cluster</devicetype>
                    <devices>
                        <device>
                            <properties>
                                <property id="uuid_3"/>
                                <property name="objname">cna1</property>
                            </properties>
                            <devicetype>cna</devicetype>
                        </device>
                        <device>
                            <properties>
                                <property id="uuid_4"/>
                                <property name="objname">cna2</property>
                            </properties>
                            <devicetype>cna</devicetype>
                        </device>
                    </devices>
                </device>
            </devices>
        </device>
    </devices>
    <links>
        <link source="uuid_3" target="uuid_4" dir="单向"/>
    </links>
</logic_topology>
```

**testbed.xml**（测试床）：

```xml
<?xml version="1.0" encoding="UTF-8"?>
<testbed>
    <devices>
        <device>
            <properties>
                <property id="uuid_1"/>
                <property name="objname">fc10</property>
            </properties>
            <devicetype>fc</devicetype>
            <devices>
                <device>
                    <properties>
                        <property id="uuid_2"/>
                        <property name="objname">cluster10</property>
                    </properties>
                    <devicetype>cluster</devicetype>
                    <devices>
                        <device>
                            <properties>
                                <property id="uuid_3"/>
                                <property name="objname">cna10</property>
                            </properties>
                            <devicetype>cna</devicetype>
                        </device>
                        <device>
                            <properties>
                                <property id="uuid_4"/>
                                <property name="objname">cna20</property>
                            </properties>
                            <devicetype>cna</devicetype>
                        </device>
                    </devices>
                </device>
            </devices>
        </device>
        <device>
            <properties>
                <property id="uuid_5"/>
                <property name="objname">fc20</property>
            </properties>
            <devicetype>fc</devicetype>
            <devices>
                <device>
                    <properties>
                        <property id="uuid_6"/>
                        <property name="objname">cluster20</property>
                    </properties>
                    <devicetype>cluster</devicetype>
                    <devices>
                        <device>
                            <properties>
                                <property id="uuid_7"/>
                                <property name="objname">cna30</property>
                            </properties>
                            <devicetype>cna</devicetype>
                        </device>
                        <device>
                            <properties>
                                <property id="uuid_8"/>
                                <property name="objname">cna40</property>
                            </properties>
                            <devicetype>cna</devicetype>
                        </device>
                    </devices>
                </device>
            </devices>
        </device>
    </devices>
    <links>
        <link source="uuid_3" target="uuid_4" dir="双向"/>
        <link source="uuid_7" target="uuid_8" dir="单向"/>
    </links>
</testbed>
```

### 7.3 执行分配

```bash
./bin/topo-match --logic examples/logic_topo.xml --testbed examples/testbed.xml
```

**输出**：

```json
{
  "testbed_name": "examples/testbed.xml",
  "mapping": {
    "uuid_1": "uuid_5",
    "uuid_2": "uuid_6",
    "uuid_3": "uuid_7",
    "uuid_4": "uuid_8"
  },
  "nodes": {
    "uuid_5": { "objname": "fc20", "device_type": "fc", "properties": {}, "status": "used", "share": false },
    "uuid_6": { "objname": "cluster20", "device_type": "cluster", "properties": {}, "status": "used", "share": false },
    "uuid_7": { "objname": "cna30", "device_type": "cna", "properties": {}, "status": "used", "share": false },
    "uuid_8": { "objname": "cna40", "device_type": "cna", "properties": {}, "status": "used", "share": false }
  }
}
```

> **匹配解析**：逻辑拓扑要求 cna1→cna2 单向链路，fc10 分支的 cna10↔cna20 是双向链路（不匹配），fc20 分支的 cna30→cna40 是单向链路（匹配成功）。

### 7.4 释放环境

将分配结果保存为 JSON 文件后释放：

```bash
# 保存结果
./bin/topo-match --logic examples/logic_topo.xml --testbed examples/testbed.xml > result.json

# 释放
./bin/topo-match --free result.json --testbed examples/testbed.xml
```

**输出**：

```
Successfully freed allocation on testbed: examples/testbed.xml
  uuid_5 (fc20) -> idle
  uuid_6 (cluster20) -> idle
  uuid_7 (cna30) -> idle
  uuid_8 (cna40) -> idle
```

### 7.5 REST API 快速上手

```bash
# 1. 启动服务
./bin/topo-match --serve --port 8080

# 2. 注册测试床
curl -X POST http://localhost:8080/api/v1/testbeds \
  -H "Content-Type: application/json" \
  -d '{"name": "tb1", "file_path": "/path/to/testbed.xml"}'

# 3. 分配环境
curl -X POST http://localhost:8080/api/v1/alloc \
  -H "Content-Type: application/json" \
  -d '{"logic_file_path": "/path/to/logic_topo.xml", "testbed_names": ["tb1"]}'

# 4. 查看测试床状态
curl http://localhost:8080/api/v1/testbeds/tb1/status

# 5. 释放环境
curl -X POST http://localhost:8080/api/v1/free \
  -H "Content-Type: application/json" \
  -d '{"alloc_result": { <上一步返回的data字段内容> }}'

# 6. 健康检查
curl http://localhost:8080/api/v1/health
```

### 7.6 属性约束案例

逻辑拓扑中指定属性约束：

```xml
<device>
    <properties>
        <property id="uuid_1"/>
        <property name="objname">fc1</property>
        <property name="version">2</property>    <!-- 要求版本=2 -->
    </properties>
    <devicetype>fc</devicetype>
</device>
```

匹配规则：
- 物理节点 `version=2` → ✅ 匹配
- 物理节点 `version=1` → ❌ 不匹配
- 物理节点无 `version` 属性 → ❌ 不匹配
- 逻辑属性 `version=""` → ✅ 不约束（跳过检查）

---

## 8. API 参考

### 8.1 REST API 端点

| 方法 | 路径 | 说明 |
|------|------|------|
| POST | `/api/v1/testbeds` | 注册测试床 |
| DELETE | `/api/v1/testbeds/:name` | 移除测试床 |
| GET | `/api/v1/testbeds` | 列出所有测试床 |
| GET | `/api/v1/testbeds/:name/status` | 查看测试床节点状态 |
| POST | `/api/v1/alloc` | 分配物理环境 |
| POST | `/api/v1/free` | 释放物理环境 |
| GET | `/api/v1/health` | 健康检查 |

### 8.2 请求/响应格式

#### 注册测试床

```json
// POST /api/v1/testbeds
// Request:
{
  "name": "tb1",
  "file_path": "/data/testbeds/tb1.xml"
}
// Response:
{
  "success": true,
  "data": "tb1"
}
```

#### 分配环境

```json
// POST /api/v1/alloc
// Request:
{
  "logic_file_path": "/data/logic/need1.xml",
  "testbed_names": ["tb1", "tb2"]
}
// Response (成功):
{
  "success": true,
  "data": {
    "testbed_name": "tb2",
    "mapping": { "l1": "p5", "l2": "p6" },
    "nodes": {
      "p5": { "objname": "fc20", "device_type": "fc", "properties": {}, "status": "used", "share": false },
      "p6": { "objname": "cluster20", "device_type": "cluster", "properties": {}, "status": "used", "share": false }
    }
  }
}
// Response (失败):
{
  "success": false,
  "error": "无空闲的测试床物理环境可用"
}
```

#### 释放环境

```json
// POST /api/v1/free
// Request:
{
  "alloc_result": {
    "testbed_name": "tb2",
    "mapping": { "l1": "p5", "l2": "p6" },
    "nodes": {
      "p5": { "objname": "fc20", "device_type": "fc", "properties": {}, "status": "used", "share": false },
      "p6": { "objname": "cluster20", "device_type": "cluster", "properties": {}, "status": "used", "share": false }
    }
  }
}
// Response:
{
  "success": true
}
```

#### 查看测试床状态

```json
// GET /api/v1/testbeds/tb1/status
// Response:
{
  "success": true,
  "data": {
    "uuid_1": { "status": "idle", "share": false, "ref_count": 0 },
    "uuid_2": { "status": "used", "share": true, "ref_count": 2 }
  }
}
```

### 8.3 CLI 参数

| 参数 | 说明 |
|------|------|
| `--logic <file>` | 逻辑拓扑 XML 文件路径 |
| `--testbed <files>` | 测试床 XML 文件路径（逗号分隔） |
| `--free <file>` | 待释放的分配结果 JSON 文件（需配合 --testbed） |
| `--serve` | 以 REST API 服务模式运行 |
| `--port <port>` | 服务端口（默认 8080，仅 serve 模式） |
| `--version` | 打印版本号 |

---

## 9. 扩展路线

| 优先级 | 方向 | 说明 |
|--------|------|------|
| 🔴 高 | 读写锁优化 | 当前全局 Mutex 串行化，Alloc 读多写少场景可用 RWMutex 提升并发 |
| 🔴 高 | 状态持久化 | 进程重启后状态丢失，需引入持久化存储（文件/DB） |
| 🟡 中 | 优雅关闭 | Server 模式需支持信号捕获 + 请求排空 |
| 🟡 中 | 最优匹配策略 | 当前返回首个匹配，可扩展为最小占用/最均衡等策略 |
| 🟡 中 | XML 字符串输入 | Server 模式支持直接 POST XML 内容，不依赖文件路径 |
| 🟢 低 | 单元测试补全 | XML 解析、model、server 层测试覆盖 |
| 🟢 低 | 并发安全测试 | allocator 层的并发 Alloc/Free 压力测试 |
| 🟢 低 | 匹配性能优化 | 大规模场景下可引入 device_type 索引、剪枝策略 |
