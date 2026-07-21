# AGENTS.md

本文件是 agent 指令索引。详细规范按使用场景拆分到 `docs/agents/` 目录下的独立文件，按需加载。

## 文件索引

| 文件 | 使用场景 | 加载时机 |
|------|---------|---------|
| [docs/agents/meta.md](docs/agents/meta.md) | 角色、执行循环、边界、Karpathy 编码原则 | **始终加载** |
| [docs/agents/workflow.md](docs/agents/workflow.md) | Skill 路由、开发工作流、Git 分支与 worktree 规范、任务分类 | 收到新任务时 |
| [docs/agents/architecture.md](docs/agents/architecture.md) | 项目架构、启动链路、请求链路、DI、优雅关闭 | 需要理解代码库时 |
| [docs/agents/commands.md](docs/agents/commands.md) | 构建、测试、lint、清理命令 | 需要执行命令时 |
| [docs/agents/go-backend.md](docs/agents/go-backend.md) | 测试契约、代码契约、Context 契约、DTO与API契约、API路由命名 | 编写或修改 Go 后端代码时 |
| [docs/agents/repo-ci.md](docs/agents/repo-ci.md) | 仓库管理、CI workflow、K8s 部署、技术债清理 | 涉及 git/CI/部署时 |
| [docs/agents/web-frontend.md](docs/agents/web-frontend.md) | Web 前端项目模型、目录结构、开发契约、联调发布 | 修改 `web/` 前端代码时 |

## 加载顺序

1. **会话开始** → 读 `meta.md`（始终生效）
2. **收到任务** → 读 `workflow.md`（确定流程和 skill）
3. **按任务类型** → 读对应场景文件：
   - 写 Go 代码 → `go-backend.md` + `architecture.md` + `commands.md`
   - 修前端 → `web-frontend.md` + `commands.md`
   - git/CI/部署 → `repo-ci.md`
   - 排障/bugfix → `workflow.md` 中的 CLS 排障步骤
