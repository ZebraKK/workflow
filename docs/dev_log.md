# Dev Log — Workflow 项目修复记录

> 关联 review: [review_0404.md](./review_0404.md)
> 修复顺序按优先级排列，每条记录包含：问题描述、变更位置、修改原因、验证方式。

---

## 待修复列表

| ID | 优先级 | 状态 | 描述 |
|---|---|---|---|
| CC2 | P0 | ✅ 已修复 | `worker()` 假并行，并行 stage 实为串行 |
| C2  | P0 | ✅ 已修复 | 异步重排队条件逻辑错误 |
| C6  | P0 | ✅ 已修复 | 并行异步阶段无法续接下一阶段 |
| C5  | P1 | 📝 已记录 | 异步回调后续阶段运行在 async worker pool（需架构改造） |
| C4  | P1 | ✅ 已修复 | CreatePipeline TOCTOU race |
| C3  | P1 | ✅ 已修复 | LaunchPipeline 浅拷贝 task 指针 |
| C1  | P2 | ✅ 已修复 | Close() 不排水丢弃未处理 Job |
| T4  | P2 | ✅ 已修复 | nil Task 测试断言错误 |
| D3  | P3 | ⏭️ 暂缓 | maxConcurrentJobs 硬编码（Phase 4 范围外） |
| Dead| P3 | ✅ 已修复 | 清理死字段 Stage.Ctx / defaultCtx / runningMode |

---

## 2026-04-04 Phase 1 — 核心逻辑 Bug 修复

---

### [CC2] worker() 假并行 → 真并发

**问题**: `parallel_stage.go:103` 的 `worker()` 在函数体内同步调用 `stp.Handle()`，外层 `go s.worker(...)` 虽开了 goroutine，但 goroutine 内部第一件事就是阻塞在 `Handle` 上，实际所有步骤串行执行，并行语义完全失效。

**变更**:
- `stage/parallel_stage.go:103-108` — 在 `worker()` 内用 `go func()` 包裹 `stp.Handle()`

**原因**: `done <- stp.Handle(...)` 必须在新 goroutine 里执行，`worker()` 才能立即返回 channel，让调用方的 `go func` 不再阻塞。

**验证**: `go test -race ./...` 全部通过

---

### [C2] 异步重排队条件逻辑错误

**问题**: `workflow.go:236` 用 `len(ids) < task.StepsCount()` 判断是否需要重排队。`len(ids)` 是 `runningID` 路径深度（静态值），`StepsCount()` 是总阶段数，两者量纲不同，几乎永远满足条件，导致已完成的 Job 被无限重排队。

**变更**:
- `workflow.go:235-249` — 删除错误的 `if/else` 分支，`StatusDone` 时直接清理 jobsStore

**原因**: `AsyncHandle` 内部 `serialAsyncHandle` 已内联调用 `serialHandle` 完成后续串行阶段，根 record 为 `Done` 意味着整个 Job 已结束，不需要重排队。

**验证**: `go test -race ./...` 全部通过

---

### [C6] 并行异步阶段完成后未设置 StatusDone

**问题**: `parallelAsyncHandle` 在确认所有步骤完成后，注释写"由外层控制"但未显式设置 `rcder.SetStatus(record.StatusDone)`。外层串行阶段轮询 record 时看到的仍是 `AsyncWaiting`，无法感知并行阶段已完成，续接永远不触发。

**变更**:
- `stage/parallel_stage.go:152-154` — 循环后添加 `rcder.SetStatus(record.StatusDone)`

**原因**: 外层 `serialAsyncHandle` 检查所有子 record 状态来判断是否继续，并行阶段必须自己将状态设为 `Done`，才能驱动外层的续接逻辑。

**验证**: `go test -race ./...` 全部通过

---

### [C5] 记录架构债务：async 续接在错误 goroutine pool 执行

**问题**: `serialAsyncHandle` 在回调处理后内联调用 `serialHandle(ctx, index+1, ...)`，后续同步阶段运行在 async worker goroutine 上，绕过 job worker pool 背压。若后续阶段耗时长，async pool 被耗尽，新回调无法处理。

**变更**:
- `stage/serial_stage.go:110-114` — 添加 `TODO(C5)` 注释，说明问题根因和正确修复方向

**原因**: 根本修复需要变更 `AsyncHandle` 接口签名（增加返回值）或引入续接 channel，属于架构级改造，Phase 1 先标注，后续 Phase 重构。

**验证**: 已记录，不影响当前测试

---

---

## 2026-04-04 Phase 2 — 并发安全修复

---

### [C4] CreatePipeline / Delete / Update TOCTOU race

**问题**: `CreatePipeline`、`DeletePipeline`、`DeletePipelineByName`、`UpdatePipeline`、`UpdatePipelineByName` 均采用"RLock 检查 → RUnlock → Lock 写入"的双锁模式，两次加锁之间存在竞争窗口：并发相同操作可绕过存在性检查，导致重复插入或操作已删除对象。

**变更**:
- `workflow_mgr.go:59-87` (`CreatePipeline`) — 移除 RLock 检查阶段，在 `Lock()` 持有后再做重名检查，check+insert 在同一临界区
- `workflow_mgr.go:110-143` (`DeletePipeline`, `DeletePipelineByName`) — 改为全程写锁，check+delete 原子化
- `workflow_mgr.go:145-176` (`UpdatePipeline`, `UpdatePipelineByName`) — 改为全程写锁，check+update 原子化

**原因**: `sync.RWMutex` 不支持锁升级（RLock → Lock），两次加锁之间必然存在窗口。正确做法是对写操作直接使用写锁。

**验证**: `go test -race ./...` 全部通过

---

### [C3] LaunchPipeline 解引用 pl 在锁外

**问题**: `LaunchPipeline` 在 `RUnlock` 后才执行 `plInstance := *pl`，并发 `UpdatePipeline` 在两者之间写入 `pl.task` 时产生数据竞争（pl 指向的内存被并发读写）。

**变更**:
- `workflow_mgr.go:183-204` (`LaunchPipeline`) — 将 `plInstance = *pl` 移至 `RLock` 持有期间执行，锁释放前完成结构体快照

**原因**: 持锁期间解引用可保证读取 Pipeline 字段时没有并发写入。快照后 `plInstance` 是独立副本，`task` interface 也是发起时的版本，不受后续 `UpdatePipeline` 影响。

**验证**: `go test -race ./...` 全部通过

---

---

## 2026-04-04 Phase 3 — 健壮性修复

---

### [C1] Close() 排水语义

**问题**: `Close()` 先关闭 `quitJobCh`/`quitAsyncCh`，workers 收到 quit 信号后立即退出。`JobCh` 中已入队但未处理的 Job 被静默丢弃，用户无任何感知。

**变更**:
- `workflow.go:28-43` — 移除 `quitJobCh`/`quitAsyncCh` 字段，添加 `isClosed atomic.Bool` 和 `closeOnce sync.Once`
- `workflow.go:82-113` (`jobStart`) — worker 循环改为 `for job := range w.JobCh`，channel 关闭且排空后自然退出
- `workflow.go:164-198` (`asyncJobStart`) — 同上改为 `for job := range w.AsyncCh`
- `workflow.go:238-262` (`Close`) — 先设 `isClosed=true`，再 `close(JobCh)` 等 job workers 排水完毕，再 `close(AsyncCh)` 等 async workers 排水完毕；`closeOnce` 保证幂等
- `workflow_mgr.go` (`LaunchPipeline`, `CallbackHandler`) — 投递前检查 `isClosed`，关闭后返回明确错误
- `workflow_test.go:157-159` — 删除对已移除 `quitJobCh` 字段的检查

**原因**: `for range` 在 channel 关闭后会继续消费剩余元素再退出，是 Go 处理"关闭后排水"的惯用模式。关闭顺序：JobCh → job workers 退出 → AsyncCh → async workers 退出，保证不会有新 AsyncJob 在 AsyncCh 关闭后产生。

**验证**: `go test -race ./...` 全部通过

---

### [T4] nil Task 测试固化了 bug

**问题**: `TestCreatePipeline_NilTask` 断言 nil Tasker 创建成功（err == nil），但 `LaunchPipeline` 调用 `task.StepsCount()` 会立即 panic，测试实际上将 bug 固化为预期行为。

**变更**:
- `workflow_mgr.go:59-68` (`CreatePipeline`) — 添加 `t == nil` 校验，返回 `"task cannot be nil"` 错误
- `workflow_test.go:191-200` (`TestCreatePipeline_NilTask`) — 断言改为 `err != nil`，并将 `defer time.Sleep` 替换为 `defer wf.Close()`

**原因**: nil task 在创建时就应被拒绝，运行时 panic 比明确 error 更难调试。

**验证**: `go test -race ./...` 全部通过

---

---

## 2026-04-04 Phase 4 — 工程完善

---

### [D4/D5] 清理死字段

**问题**: 三个字段声明后从未读取或写入：`Stage.Ctx string`（带 TODO 注释）、`Pipeline.defaultCtx string`、`Pipeline.runningMode string`，造成代码噪音，误导阅读者以为这些字段有实际语义。

**变更**:
- `stage/stage.go:21` — 删除 `Ctx string` 字段
- `workflow_mgr.go:26-32` (`Pipeline`) — 删除 `defaultCtx` 和 `runningMode` 字段
- `workflow_mgr.go:69-76` (`CreatePipeline`) — 删除对应的初始化字面量

**原因**: 死字段会让维护者以为它们被使用，徒增认知负担。需要时再引入，不要提前占位。

**验证**: `go test -race ./...` 全部通过

---

### [T2] 补充缺失测试 + 修复 WaitGroup race

**问题**:
1. `DeletePipelineByName`、`UpdatePipelineByName`、`Close()` 均未被测试覆盖
2. `NewWorkflow` 中使用 `go wf.jobStart()` 异步启动，`jobWg.Add(1)` 在子 goroutine 中执行，若 `Close()` 在 `Add` 前调用 `Wait()` 则形成真实 data race（race detector 检出）

**变更**:
- `workflow_test.go` — 新增 6 个测试：
  - `TestDeletePipelineByName` / `TestDeletePipelineByName_NotFound`
  - `TestUpdatePipelineByName` / `TestUpdatePipelineByName_NotFound`
  - `TestClose_DrainsPendingJobs` — 验证 Close() 等待已入队 job 执行完毕
  - `TestClose_RejectsNewJobs` — 验证 Close() 后 LaunchPipeline 返回 error
  - `TestClose_Idempotent` — 验证 Close() 多次调用不 panic
- `workflow.go:74-77` (`NewWorkflow`) — `go wf.jobStart()` → `wf.jobStart()`，同步调用确保所有 `Add(1)` 在 `NewWorkflow` 返回前完成

**原因**: `sync.WaitGroup` 规范：`Add` 必须在 `Wait` 调用前完成，或者有明确的同步点。异步启动 `jobStart` 会打破这个不变量。同步调用不影响性能，因为 `jobStart` 内部立即开 goroutine 然后返回。

**验证**: `go test -race ./...` 全部通过，race detector 干净

---

---

## 2026-04-13 持久化层 + 接口变更

---

### [新增] Store 接口 + JobRecord + MemoryStore

**变更**:
- `store.go`（新建）— `Store` 接口（Save/Load/Delete/ListByStatus）、`JobRecord` 结构体、`MemoryStore` 内置实现、`newJobRecord()`、`restoreJob()` 辅助函数

**原因**: 持久化层作为可插拔接口，使用方自选后端（Redis/DB）；内置 `MemoryStore` 供测试和开发使用。

---

### [新增] RecordSnapshot 序列化

**变更**:
- `record/record.go` — 新增 `RecordSnapshot` 结构体（含 JSON 标签），`Snapshot()` 递归生成快照，`RestoreRecord()` 从快照重建 Record 树

**原因**: Record 含 `sync.RWMutex` 不可直接序列化；RecordSnapshot 是纯数据结构，可安全持久化。

---

### [变更] Job.ctx 类型 + LaunchPipeline 签名

**变更**:
- `workflow_mgr.go` — `Job.ctx` 从 `interface{}` 改为 `json.RawMessage`；`LaunchPipeline` 签名从 `(id, ctx) error` 改为 `(id string, ctx json.RawMessage) (string, error)`，返回 jobID
- `workflow_test.go` — 所有调用更新为新签名；`example/example.go` 同步更新

**原因**: ctx 改为 `json.RawMessage` 使其天然可序列化，调用方负责序列化（Option A）；返回 jobID 满足需求中"响应 job id 供后续查询"的要求。

---

### [新增] Workflow 持久化钩子 + 启动恢复

**变更**:
- `workflow.go` — `Workflow` 新增 `store Store` 字段；`NewWorkflow` 新增 `store Store` 参数（nil=纯内存模式）；`runJob`/`runAsyncJob` 在关键节点调用 `store.Save`/`store.Delete`；`recoverPendingJobs()` 在启动时从 store 加载 async_waiting job 回内存

**持久化时机**:
- `LaunchPipeline` 投入队列后: `store.Save(created)`
- `AsyncWaiting`: `store.Save(updated Record snapshot)`
- `Done/Failed`: `store.Delete`

**恢复逻辑**: 按 PipelineID 找已注册 Pipeline → `RestoreRecord` 重建 Record 树 → 放入 jobsStore 等待回调；Pipeline 未注册则 Warn 跳过。

**验证**: `go test -race ./...` 全部通过，race detector 干净

---

<!-- 修复记录从这里开始，格式如下：

## [日期] [ID] 修复标题

**问题**: 描述问题

**变更**:
- `文件:行号` — 具体改动

**原因**: 为什么这样改

**验证**: 运行 `go test ./...` 结果

-->
