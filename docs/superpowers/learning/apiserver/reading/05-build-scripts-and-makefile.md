# 05 Makefile、脚本、为什么这样设计

## 1. 为什么顶层只保留一个 Makefile

因为当前项目规模还没大到需要多层 Makefile。

如果现在就把顶层拆成很多 `Makefile.*`：
- 看起来更复杂
- 学习成本更高
- 对当前收益不大

所以当前的取舍是：
- 顶层只保留一个 `Makefile`
- 复杂逻辑下沉到 `tools/hack/`

这比“所有逻辑都堆在 Makefile 里”更稳，也比“过早多层 Makefile”更干净。

## 2. 为什么要有 `tools/hack/common.sh`

因为脚本之间有共享概念：
- 根目录
- build 目录
- 二进制名映射
- codegen 版本
- git version / commit / build date

这些逻辑如果每个脚本都复制一份，会很快漂。

所以要抽公共层。

## 3. 为什么现在用 `_output` 和 `BUILD_DIR`

这是我们前面特意收过的。

规则是：
- 具体目录名：`_output`
- 脚本和 Makefile 变量名：`BUILD_DIR`

为什么这么分？

因为：
- `_output` 表示真实目录
- `BUILD_DIR` 表示“构建目标目录”这个概念

这样语义更清楚，也不会再混 `out/output/_output`。

## 4. 为什么要有 `check-tools`

因为工程不能靠人脑记依赖。

项目应该自己能告诉你：
- 缺什么工具
- 为什么缺
- 下一步该怎么补

## 5. 为什么要有 `version`

因为构建产物最好不是匿名的。

所以 `build.sh` 里用 `ldflags` 注入：
- `GitVersion`
- `GitCommit`
- `BuildDate`

这不是炫技，而是非常实际的工程需求。
