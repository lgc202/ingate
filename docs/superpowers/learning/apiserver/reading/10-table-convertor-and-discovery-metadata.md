# 10 TableConvertor 和 Discovery 元数据

这篇文档解决两个经常一起出现的问题：

1. 为什么资源列表可以返回业务化表格
2. 为什么 `/apis/<group>/<version>` 里会有 `shortNames`、`categories` 这些元数据

这两件事都属于：

**让资源 API 不只是“能用”，而且“更像正式 Kubernetes 风格 API”。**

## 先说结论

在当前 `Ingate apiserver` 里：

1. `TableConvertor` 决定资源列表怎样变成 `Table`
2. `ShortNames()` / `Categories()` 决定 discovery 里怎样描述资源
3. 这两部分都挂在资源 storage 上

也就是说，资源的“存储入口”和“对外展示元数据”是在同一层汇合的。

## 为什么不用默认 Table

默认 Table 的问题是：

1. 不懂 `Gateway/Route/Backend/AuthPolicy/TrafficPolicy` 的业务语义
2. 不知道哪些列最值得展示
3. 最后要么列很弱，要么几乎没有业务价值

所以我们自己做了业务化列：

- `Gateway`: `Listeners`、`Hostnames`
- `Route`: `Parents`、`Rules`、`Hostnames`
- `Backend`: `Type`、`Port`、`Endpoints`
- `AuthPolicy`: `Type`、`Targets`
- `TrafficPolicy`: `Timeout`、`RateLimit`

## 代码从哪里看

### 通用 Table 逻辑
- [table.go](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/registry/common/table.go)

这层负责：

1. 接收资源对象或资源列表
2. 逐个对象调用 `rowFn`
3. 组装成 `meta.k8s.io/v1 Table`
4. 处理 `NoHeaders`
5. 复制 list metadata，比如 `resourceVersion`

这层不懂 `Gateway`、`Route` 这些业务对象本身。
它只负责：

**把“对象 -> 行”的规则装进标准 Table 结构。**

### Gateway 组的表列和取值
- [gateway table](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/table/table.go)

### Policy 组的表列和取值
- [policy table](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/registry/policy/table/table.go)

这里的代码负责：

1. 定义列名
2. 定义列类型
3. 从对象里提取每一列的值

## 为什么 TableConvertor 要挂在 storage 上

看这些文件：

- [gateway storage](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/gateway/storage/storage.go)
- [route storage](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/route/storage/storage.go)
- [backend storage](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/registry/gateway/backend/storage/storage.go)
- [authpolicy storage](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/registry/policy/authpolicy/storage/storage.go)
- [trafficpolicy storage](/Users/guangcaili/workplace/code/lgc202/ingate/internal/controlplane/apiserver/registry/policy/trafficpolicy/storage/storage.go)

你会看到类似：

```go
TableConvertor: commonregistry.NewTableConvertor(...)
```

这样做的原因是：

1. storage 最清楚自己服务的是哪种资源
2. storage 最适合声明这个资源的展示元数据
3. 这和 `CreateStrategy / UpdateStrategy / DeleteStrategy` 放在一起，结构最完整

## discovery 元数据为什么也在这里

同样在这些 storage 文件里，你会看到：

```go
func (*REST) Categories() []string
func (*REST) ShortNames() []string
```

它们决定了 `/apis/<group>/<version>` 里的：

- `shortNames`
- `categories`

## 为什么 shortNames 和资源名要集中定义

看这两个文件：

- [gateway resources](/Users/guangcaili/workplace/code/lgc202/ingate/pkg/apis/gateway/v1alpha1/resources.go)
- [policy resources](/Users/guangcaili/workplace/code/lgc202/ingate/pkg/apis/policy/v1alpha1/resources.go)

这里集中定义了：

- resource 名
- singular 名
- status 子资源名
- short name
- category

这样做的原因很直接：

1. 避免魔法字符串散在 storage/provider 里
2. API 变更时只有一个地方要改
3. 资源 discovery 和 REST 安装链能共用同一套常量

这正好符合你前面提的要求：

**尽量不要出现太多魔法字符串、数字。**

## 这条链完整长什么样

你可以把它记成：

```text
resource constants
  -> storage declares TableConvertor/ShortNames/Categories
  -> apiserver installs resource storage
  -> discovery 能看到 shortNames/categories
  -> list 请求带 Table Accept 头时返回业务列
```

## 它和默认 JSON/list 的关系

要分清：

1. JSON list 还是主返回
2. Table 是另一种表示形式
3. discovery 元数据又是另一层“自描述能力”

所以这三件事虽然在代码里靠得近，但语义不同：

- `list JSON`: 真正数据
- `Table`: 面向列表查看的视图
- `discovery`: API 自描述元数据

## 你读完这篇后，应该能回答

1. 为什么我们要自己写 `TableConvertor`
2. 为什么 `ShortNames()` 和 `Categories()` 也在 storage 层
3. 为什么资源常量要单独放 `resources.go`
4. 为什么这套设计会比把字符串散写在各处更稳
