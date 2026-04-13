# 代码修复清单

基于代码审查报告，这是需要修复的问题清单，按优先级排序。

---

## 🔴 P0 - 严重问题（必须立即修复）

### ✅ 1. 接口方法命名不一致 - **已完成**

**问题**: `Actioner`接口方法在代码中是`Handle`，文档中是`StepActor`

**文件**:
- [x] `docs/CONTEXT_PARAMETER_GUIDE.md` - 更新文档 ✅
- [x] `docs/TIMEOUT_CONTROL_GUIDE.md` - 更新文档 ✅
- [x] `docs/TIMEOUT_FEATURE_SUMMARY.md` - 更新文档 ✅
- [x] `docs/CHANGES_SUMMARY.md` - 更新文档 ✅
- [x] `docs/CODE_REVIEW_REPORT.md` - 更新报告 ✅

**决策**: ✅ 使用`Handle`（保持代码不变，只更新文档）

**完成日期**: 2026-03-06

---

### ✅ 2. Step构造函数默认超时逻辑错误 - **已完成**

**问题**: 传入`timeout=0`会被强制改为10秒，无法创建不超时的Step

**文件**:
- [x] `step/step.go:32-47` - 修复NewStep函数 ✅
- [x] `step/step.go:84-91` - 修复Handle方法 ✅
- [x] `step/step.go:133-140` - 修复AsyncHandle方法 ✅

**修复内容**:
- 移除了强制设置10秒默认值的逻辑
- timeout=0时不使用超时控制，直接执行
- timeout>0时使用超时控制
- 将nil检查改为panic（构造函数参数验证）

**完成日期**: 2026-03-06

---

### ✅ 3. 状态字符串硬编码 - **已完成**

**问题**: 多处使用硬编码状态字符串，应该使用`record`包的常量

**文件**:
- [x] `workflow.go` - 所有状态硬编码已替换 ✅
- [x] `stage/serial_stage.go` - Serial stage已替换 ✅
- [x] `stage/parallel_stage.go` - Parallel stage已替换 ✅
- [x] `step/step.go` - Step已替换 ✅
- [x] `record/record.go` - Record已替换 ✅

**统一决策**:
- [x] 使用`StatusDone`，移除了所有`"completed"`引用 ✅
- [x] 添加了`workflow/record`包导入到`workflow.go` ✅

**测试**:
- [x] 所有测试通过 ✅
- [x] 更新了step_test.go以适应panic行为 ✅

**完成日期**: 2026-03-06

---

## 🟠 P1 - 重要问题（应该尽快修复）

### ✅ 4. 并发竞态条件 - Record - **已完成**

**问题**: `Record.AddRecord()`和状态更新没有并发保护

**修复内容**:
- [x] `record/record.go` - 添加`sync.RWMutex` ✅
- [x] 实现`SetStatus()`和`GetStatus()`方法 ✅
- [x] 实现`SetStartAt()`和`SetEndAt()`方法 ✅
- [x] 实现`SetAsyncRecord()`和`GetAsyncRecord()`方法 ✅
- [x] 实现`GetRecord()`和`GetRecordsLen()`方法 ✅
- [x] 更新`step/step.go` - 使用新方法 ✅
- [x] 更新`stage/serial_stage.go` - 使用新方法 ✅
- [x] 更新`stage/parallel_stage.go` - 使用新方法 ✅
- [x] 更新`workflow.go` - 使用新方法 ✅
- [x] 更新`workflow_test.go` - 使用新方法 ✅

**测试验证**:
- [x] `go test ./...` - 全部通过 ✅
- [x] `go test -race ./...` - 零竞态条件 ✅

**完成日期**: 2026-03-06

---

### ✅ 5. 错误处理不完整 - **已完成**

**问题**: 多处错误处理不规范

**修复内容**:
- [x] `step/step.go` - actor为nil改为panic ✅ (P0-2中已修复)
- [x] `serial_stage.go` - 添加Record nil检查 ✅
- [x] `parallel_stage.go` - 添加Record nil检查 ✅
- [x] `serial_stage.go:92-96` - 检查GetRecord返回值 ✅
- [x] `parallel_stage.go:133-137` - 检查GetRecord返回值 ✅

**遵循的标准**:
- ✅ 构造函数参数验证：panic
- ✅ 运行时错误：返回error并记录日志
- ✅ 所有GetRecord调用都检查nil

**完成日期**: 2026-03-06

---

### ✅ 6. Worker优雅关闭不完整 - **已完成**

**问题**: `Close()`没有等待Worker完成

**修复内容**:
- [x] `workflow.go:28-42` - 添加`jobWg`和`asyncWg`字段 ✅
- [x] `workflow.go:79-114` - jobStart使用jobWg ✅
- [x] `workflow.go:166-201` - asyncJobStart使用asyncWg ✅
- [x] `workflow.go:270-294` - Close等待所有WaitGroup ✅

**优雅关闭流程**:
1. 发送quit信号到所有worker
2. 等待所有job workers完成 (jobWg.Wait())
3. 等待所有async workers完成 (asyncWg.Wait())
4. 关闭channels
5. 完成

**测试验证**:
- [x] 所有测试通过 ✅
- [x] 竞态检测通过 ✅

**完成日期**: 2026-03-06

---

### ✅ 7. Logger测试缺失 - **已完成**

**问题**: logger包测试覆盖率0%

**修复内容**:
- [x] 创建`logger/logger_test.go` ✅
- [x] 测试`SlogLogger`的所有方法 ✅
- [x] 测试`NoOpLogger` ✅
- [x] 测试`With()`方法 ✅
- [x] 添加13个综合测试用例 ✅
- [x] 测试级别过滤、多参数日志等 ✅

**结果**: 覆盖率达到 100% ✅

**完成日期**: 2026-03-09

---

## 🟡 P2 - 中等问题（建议修复）

### ✅/❌ 8. 日志级别使用不规范

**问题**: 部分日志级别不恰当

**文件**:
- [ ] `workflow_mgr.go:102` - GetPipelineByName失败改为Debug
- [ ] 审查所有Warn级别日志
- [ ] 审查所有Error级别日志

**标准**:
- Error: 系统错误、异常
- Warn: 警告（非预期但可恢复）
- Info: 重要业务操作
- Debug: 调试信息

---

### ✅/❌ 9. Magic Number定义为常量

**文件**:
- [ ] `parallel_stage.go:13` - maxConcurrentJobs改为可配置
- [ ] `workflow_mgr.go:254` - 超时时间改为常量
- [ ] 定义常量包或在Config中配置

---

### ✅/❌ 10. 命名不一致

**问题**: 变量命名风格不统一

**文件**:
- [ ] 统一Record变量名：`rcder` vs `record` vs `rcrd` → 选择一个
- [ ] 统一方法命名：`Handle` vs `AsyncHandle`
- [ ] 更新代码风格指南

---

### ✅/❌ 11. 删除空文件

**文件**:
- [ ] 删除`example/myStore.go`或实现内容

---

## 🔵 P3 - 低优先级（可选）

### ✅/❌ 12. 注释语言统一

**决策**:
- [ ] 选项A: 全部英文（开源项目推荐）
- [ ] 选项B: 全部中文（内部项目）

**文件**: 所有`.go`文件

---

### ✅/❌ 13. 字段导出规则统一

**文件**:
- [ ] 审查所有struct字段
- [ ] 不应导出的改为小写
- [ ] 应导出的添加GoDoc

---

### ✅/❌ 14. 添加GoDoc注释

**文件**: 所有导出的类型和函数

**标准**:
- 每个导出的类型都有注释
- 每个导出的函数都有注释
- 注释描述功能，不是实现

---

## 功能增强（新功能）

### ✅/❌ 15. 实现Job取消机制

**需求**: 支持主动取消正在运行的Job

**文件**:
- [ ] `workflow.go` - 添加context支持
- [ ] `workflow_mgr.go` - 添加`CancelJob()`方法
- [ ] 所有Handle方法支持context取消
- [ ] 测试取消功能

---

### ✅/❌ 16. 实现重试机制

**需求**: README提到的重试功能

**文件**:
- [ ] 定义重试配置（次数、间隔、策略）
- [ ] 实现重试逻辑
- [ ] 测试各种重试场景

---

### ✅/❌ 17. 性能优化 - ID生成

**问题**: SHA256生成ID性能开销大

**选项**:
- [ ] 使用UUID
- [ ] 使用Snowflake ID
- [ ] Benchmark对比

---

### ✅/❌ 18. 添加监控指标

**文件**:
- [ ] 添加Prometheus metrics
- [ ] 添加健康检查接口
- [ ] 记录Job执行时间
- [ ] 记录成功/失败率

---

## 测试增强

### ✅/❌ 19. 提高测试覆盖率

**目标**: 所有包 > 80%

**当前**:
- workflow: 72.3% → 目标85%
- logger: 0% → 目标80%
- record: 100% ✅
- stage: 87.7% ✅
- step: 95.2% ✅

**文件**:
- [ ] `workflow_test.go` - 添加Close测试
- [ ] `logger/logger_test.go` - 新建测试文件

---

### ✅/❌ 20. 竞态条件测试

**命令**:
```bash
go test -race ./...
```

**要求**:
- [ ] 所有测试在race模式下通过
- [ ] 修复所有检测到的竞态条件

---

## 文档更新

### ✅/❌ 21. 更新现有文档

**文件**:
- [ ] `docs/CONTEXT_PARAMETER_GUIDE.md` - 修正接口名
- [ ] `docs/TIMEOUT_CONTROL_GUIDE.md` - 修正默认值说明
- [ ] `docs/README.md` - 更新TODO列表

---

### ✅/❌ 22. 创建新文档

**文件**:
- [ ] `docs/API.md` - API文档
- [ ] `docs/ARCHITECTURE.md` - 架构文档
- [ ] `docs/CONTRIBUTING.md` - 贡献指南
- [ ] `docs/CHANGELOG.md` - 变更日志

---

## CI/CD改进

### ✅/❌ 23. 增强CI检查

**文件**: `.github/workflows/`

**检查项**:
- [ ] `go test -race`
- [ ] `go vet`
- [ ] `golangci-lint`
- [ ] 覆盖率检查
- [ ] 文档生成

---

## 进度追踪

### 完成统计

- P0问题: 3/3 (100%) ✅ **已完成**
- P1问题: 4/4 (100%) ✅ **已完成**
- P2问题: 0/4 (0%)
- P3问题: 0/2 (0%)
- 功能增强: 0/4 (0%)
- 测试增强: 0/2 (0%)
- 文档更新: 0/2 (0%)
- CI/CD: 0/1 (0%)

**总进度**: 7/22 (32%)

**最近更新**: 2026-03-09 - P1问题全部完成！测试覆盖率大幅提升！

---

## 使用说明

1. 复制这个清单作为你的工作计划
2. 每完成一项，将`[ ]`改为`[x]`
3. 在每项后面添加完成日期和备注
4. 定期更新进度统计

---

**创建日期**: 2026-03-06
**最后更新**: 2026-03-06
**负责人**: 待分配
