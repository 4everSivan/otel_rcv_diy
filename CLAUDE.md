# Claude Code

@constitution.md
@AGENTS.md

本文件描述 Claude Code 在此项目中的专属行为。跨工具共享的开发原则见 `constitution.md`，项目事实见 `AGENTS.md`。

---

## 1. 工作流程

### 1.1 修改代码前

1. 确认目标 module 和影响范围。
2. 阅读相关 `docs/` 文档理解设计意图。
3. 检查 `go.work` 确认模块可见性。

### 1.2 修改代码后

1. `go build ./...` 确保编译通过。
2. `go vet ./...` 确保无静态分析警告。
3. `go test ./...` 确保已有测试不回归。
4. 若修改 `metadata.yaml`，重新运行 mdatagen。

### 1.3 提交前

- 检查 diff 是否只含目标变更。
- 确认提交信息符合 Conventional Commits。
- 不提交构建产物 (`.gitignore` 已覆盖)。

---

## 2. 工具使用偏好

- **代码搜索**: 优先用 `mcp__semble__search` 搜索项目代码；仅当需要精确字符串匹配时用 Grep。
- **编辑**: 优先用 Edit 做精细化替换；用 Write 做整文件覆写。
- **构建/测试输出**: 输出长时用 context-mode 工具处理，避免原始输出占用上下文。
- **并行探索**: 多个独立文件用 Agent 并行阅读。

---

## 3. Subagent 委派

- **Explore agent**: 只读搜索、跨文件查找、调用链分析。按任务广度选 `medium`（单模块）或 `very thorough`（跨模块）。
- **Plan agent**: 复杂架构设计或大重构前做方案收敛。
- **默认只读**: 仅任务明确授权且写入文件互不重叠时，才允许 subagent 修改文件。

---

## 4. 项目特定惯例

- Go 代码注释优先英文（与 OTel 上游保持一致）；文档/README 可用中文。
- 修改 `patronireceiver/metadata.yaml` 后必须运行 mdatagen 重新生成 `internal/metadata/`。
- `internal/metadata/generated_*.go` 为自动生成文件，不手动编辑。
- 配置 squash 模式: `Config` 结构体通过 `mapstructure:",squash"` 平铺子结构，用户 YAML 配置无需嵌套。
