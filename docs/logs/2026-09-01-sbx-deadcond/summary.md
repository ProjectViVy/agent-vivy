# SBX-DEADCOND: 修复 isDangerousCommand 递归强删死条件

日期：2026-09-01 ｜ 分支：`feat/vc1a-bash-tool` ｜ worktree：`agent-vivy-vc0`

## What changed

`internal/runtime/sandbox_manager.go` 的 `isDangerousCommand` 是 danger-full-access
模式下 execute/commandline 路径的独立防线（bash 工具另有 deny 表）。原实现要求
**同一个 argv 条目**同时充当 flag 与目标：

```go
if (arg == "-rf" || arg == "-fr" || arg == "/f" || arg == "/s") &&
    (arg == "/" || arg == "*" || arg == ".") { return true }
```

单个参数不可能既是 `-rf` 又是 `/`，条件永假——`rm -rf /` 检测形同虚设。

修复 = 分两趟收集再交叉判断（TODO 行点名的修法）：

- `rm`：一趟收集递归信号（`-r`/`-rf`/`-fr`/`-Rf` 短旗标簇、`--recursive`）与
  根状目标，二者齐备即拦；`--no-preserve-root` 直接拦（它存在的唯一目的就是
  打开根删除）。
- `del`/`rd`/`rmdir`（Windows）：`/s` 为递归信号，与根状目标交叉。
- 新增 `isRootLikeDeleteTarget`：识别文件系统根（`/`、引号包裹、尾随空白）、
  根通配（`/*`、`C:\*`）、当前/上级目录整体（`.`、`..`）、home（`~`、`~/*`）、
  盘符根（`C:\`、`C:/`、`c:`）。统一剥引号、反斜杠归一、TrimRight 后判定；
  深于盘根的具体路径（`c:/temp/x`）与普通工作区目标（`build`、`node_modules/pkg`）
  一律放行——该防线只在 danger 模式生效， confined 模式本就走白名单，不因
  本修复收紧正常清理类命令。

## Deliberately not done

- 不做完整 shell 解析（管道/变量/嵌套引号）：定位是"显然危险模式"启发式网，
  与 bash deny 表分层；解析级防护归 bash 工具路径。
- 不把 `rm -rf *`（cwd 通配）单独放行或加白名单机制：保留原设计目标集，
  拦截仍限于 danger 模式。

## 验收口径

- 修复前：`rm -rf /` 通过 `ConfineCommandWithMode(danger)`（防线失守）。
- 修复后：`rm -rf /`、`rm -r /`、`del /s *`、`rd /s /q .` 等 23 个危险组合
  全部拒绝，11 个正常删除/构建清理组合全部放行（见 verification.md）。
