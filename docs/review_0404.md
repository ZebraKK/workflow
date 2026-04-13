# Workflow 项目 Code Review（2026-04-04）

## 项目概览

Go 实现的轻量级工作流编排引擎，零外部依赖（纯标准库）。核心分层：

```
Workflow → Pipeline → Job → Stage (串行/并行) → Step (叶节点)
```

支持：串行执行、并行执行（信号量限流）、异步回调续接。

---

## 架构理解

### 执行流程
1. `CreatePipeline` 注册任务树（Pipeline = name + Tasker）
2. `LaunchPipeline` 创建 Job，放入 `JobCh` channel
3. Worker goroutine 消费 `JobCh`，调用 `task.Handle()`，递归驱动 Stage/Step
4. 遇到异步步骤时，Job 状态变为 `AsyncWaiting`，留在 `jobsStore`
5. 外部调用 `CallbackHandler(runningID, resp)` 恢复执行，放入 `AsyncCh`
6. Async worker 解析 `runningID`，找到对应位置，续接后续阶段

### Record ID 体系
层次化字符串 ID（如 `abc123-0-1`），编码了执行路径，用于异步回调时定位执行位置。"最后一个数字段递增"规则不直观，文档不足。

### 核心接口
```go
// Tasker — Pipeline 中的任务节点（Stage 实现此接口）
type Tasker interface {
    IsAsync() bool
    StepsCount() int
    Handle(ctx interface{}, rcder *record.Record, logger Logger) error
    AsyncHandle(ctx interface{}, resp interface{}, runningID string,
                ids []int, stageIndex int, rcder *record.Record, logger Logger)
}

// Step 叶节点动作
type Actioner interface { Handle(ctx interface{}) error }
type AsyncActioner interface { AsyncHandle(ctx interface{}, resp interface{}) error }
```

---

## 问题清单

### Phase 1 — 核心逻辑 Bug（影响功能正确性）

#### CC2: `worker()` 是假并行 ⚠️ 最严重
**文件**: `stage/parallel_stage.go:103`
```go
func (s *Stage) worker(stp steper, input interface{}, rcder *record.Record, logger Logger) <-chan error {
    done := make(chan error, 1)
    done <- stp.Handle(input, rcder, logger)  // 在返回 channel 之前就阻塞调用方 goroutine！
    return done
}
```
`stp.Handle()` 在函数体内同步执行，外层 `go s.worker(...)` 的 goroutine 并不是在 `worker` 内部并发的——它直到 `Handle` 返回才会结束。实际所有步骤串行执行。

**修复**: 在 `worker` 内开新 goroutine：
```go
go func() { done <- stp.Handle(input, rcder, logger) }()
```

#### C2: 异步重排队条件逻辑错误
**文件**: `workflow.go:236`
```go
if len(ids) < asyncJob.Job.Pipeline.task.StepsCount() { /* re-queue */ }
```
`len(ids)` 是当前回调的路径深度（固定值），`StepsCount()` 是总阶段数。两者量纲不同，条件几乎永远为真，导致无限重排队。

#### C6: 并行异步阶段永远无法继续下一阶段
**文件**: `stage/parallel_stage.go`
注释"由外层控制"，但外层的重排队逻辑即是 C2 的错误逻辑，续接实际上永远不会发生。

#### C5: 异步回调后的同步阶段运行在 async worker pool 上
**文件**: `stage/serial_stage.go:113`
```go
// serialAsyncHandle 内
t.serialHandle(ctx, index+1, rcder, logger)  // 直接同步调用，运行在 async goroutine 上
```
后续同步阶段占用 async worker，绕过 job worker pool 的背压控制，可能导致 async pool 耗尽。

---

### Phase 2 — 并发安全

#### C4: CreatePipeline TOCTOU race
**文件**: `workflow_mgr.go:65`
```go
w.muPl.RLock()
_, exists := w.pipelineMapWithName[name]  // 读锁检查重名
w.muPl.RUnlock()
// ← 竞争窗口
w.muPl.Lock()
w.pipelineMap[id] = &pl  // 写锁插入
```
两次加锁之间存在窗口，并发创建同名 Pipeline 可以绕过重名检查。

**修复**: 全程持有写锁。

#### C3: LaunchPipeline 浅拷贝 Pipeline
**文件**: `workflow_mgr.go:198`
```go
plInstance := *pl  // 浅拷贝结构体，task interface 仍是共享指针
```
并发调用 `UpdatePipeline` 时，运行中的 Job 会看到新 task，产生数据竞争。

---

### Phase 3 — 健壮性

#### C1: Close() 不排水，未处理的 Job 被丢弃
**文件**: `workflow.go:265`
`Close()` 关闭 quit channel，Worker 退出后再关闭 `JobCh`/`AsyncCh`。已投入 channel 但未被消费的 Job 会被静默丢弃。

#### T4: 测试断言 nil Task 合法（固化了 bug）
**文件**: `workflow_test.go:191`
```go
err := wf.CreatePipeline("test-pipeline", nil)
// 断言 err == nil
```
nil Tasker 在 `LaunchPipeline` 时会 panic。测试应断言返回 error。

---

### Phase 4 — 工程完善

#### D1: 无 context.Context 传播
`ctx interface{}` 是业务上下文，不支持 Go 标准的取消/超时传播。无法从外部取消运行中的 Job。

#### D2: 重试/回滚/跳过未实现
README 提及，但代码中无任何实现。

#### D3: maxConcurrentJobs 硬编码
**文件**: `stage/parallel_stage.go:13`
```go
const maxConcurrentJobs = 5
```
不可配置，无法按 Stage 或 Workflow 级别调整。

#### 死字段（声明但从不使用）
- `Stage.Ctx string` — `stage/stage.go:21`
- `Pipeline.defaultCtx` — `workflow_mgr.go`
- `Pipeline.runningMode` — `workflow_mgr.go`

#### CC1: AddStep 非线程安全
`stage.Steps` 是公开 slice，`AddStep` 与 `Handle` 并发调用时有数据竞争。目前靠约定保证，未做代码层面的保护。

#### 测试覆盖率缺口
| 函数 | 覆盖率 |
|---|---|
| `Close()` | 0% |
| `DeletePipelineByName` | 0% |
| `UpdatePipelineByName` | 0% |
| `runAsyncJob` | 43.5% |
| `serialAsyncHandle` | 65.4% |
| `parallelAsyncHandle` | 70.8% |

#### 示例代码不可运行
`example/example.go` 使用 `package example` 但定义了 `func main()`，无法作为独立程序编译。

---

## 优点

- 零外部依赖，纯标准库
- `race detector` 当前通过（但有隐患）
- `record.Record` 并发读写保护规范
- `Logger` 接口抽象干净，易于替换
- Worker goroutine 有 panic recovery，进程不会被单个任务搞挂
- Step 级别 timeout 实现正确

---

## 修复追踪

见 [dev_log.md](./dev_log.md)
