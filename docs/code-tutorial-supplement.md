# 代码教程补充 - 工程化结构与上下文详解

> 本文档面向有 Python 基础的开发者，用 Python 对比解释 Go 语法，重点讲解工程化封装和 Context 机制。

---

## 目录

1. [Go vs Python 语法速查表](#1-go-vs-python-语法速查表)
2. [工程化结构：从底层到顶层的封装](#2-工程化结构从底层到顶层的封装)
3. [Context 上下文详解](#3-context-上下文详解)
4. [调用链路完整追踪](#4-调用链路完整追踪)

---

## 1. Go vs Python 语法速查表

| 概念      | Python                     | Go                            |
| --------- | -------------------------- | ----------------------------- |
| 包声明    | 文件夹即包                 | `package xxx`                 |
| 导入      | `from x import y`          | `import "x/y"`                |
| 公开/私有 | `_name` 约定               | 首字母大小写决定              |
| 函数定义  | `def func(a: str) -> int:` | `func Func(a string) int {}`  |
| 多返回值  | `return a, b`              | `return a, b`                 |
| 错误处理  | `try/except`               | `if err != nil`               |
| 类/结构体 | `class Foo:`               | `type Foo struct {}`          |
| 方法      | `def method(self):`        | `func (f *Foo) Method() {}`   |
| 构造函数  | `def __init__(self):`      | `func NewFoo() *Foo {}`       |
| 可变参数  | `*args, **kwargs`          | `args ...Type`                |
| 列表/切片 | `list = [1, 2, 3]`         | `slice := []int{1, 2, 3}`     |
| 字典/映射 | `dict = {"a": 1}`          | `m := map[string]int{"a": 1}` |
| None/nil  | `None`                     | `nil`                         |
| 接口      | `ABC` 抽象类               | `type Interface interface {}` |

### 详细对比示例

#### 结构体与方法

```python
# Python
class Client:
    def __init__(self, timeout=30):
        self.timeout = timeout
        self.http_client = requests.Session()

    def do(self, request):
        return self.http_client.send(request)
```

```go
// Go
type Client struct {
    timeout    time.Duration
    httpClient *http.Client
}

func NewClient(timeout time.Duration) *Client {
    return &Client{
        timeout:    timeout,
        httpClient: &http.Client{Timeout: timeout},
    }
}

func (c *Client) Do(req *http.Request) (*http.Response, error) {
    return c.httpClient.Do(req)
}
```

#### 错误处理

```python
# Python
try:
    result = risky_operation()
except SomeError as e:
    print(f"Error: {e}")
    return None
```

```go
// Go
result, err := riskyOperation()
if err != nil {
    fmt.Printf("Error: %v\n", err)
    return nil, err
}
```

#### 可变参数与 Functional Options

```python
# Python - 使用 **kwargs 实现可选参数
def new_client(timeout=30, retry=3, debug=False):
    client = Client()
    client.timeout = timeout
    client.retry = retry
    client.debug = debug
    return client

# 调用
client = new_client(timeout=60, debug=True)
```

```go
// Go - 使用 Functional Options 模式
type clientOptions struct {
    timeout time.Duration
    retry   int
    debug   bool
}

type ClientOption func(*clientOptions)

func WithTimeout(d time.Duration) ClientOption {
    return func(o *clientOptions) {
        o.timeout = d
    }
}

func WithRetry(n int) ClientOption {
    return func(o *clientOptions) {
        o.retry = n
    }
}

func WithDebug(enabled bool) ClientOption {
    return func(o *clientOptions) {
        o.debug = enabled
    }
}

func NewClient(opts ...ClientOption) *Client {
    // 1. 设置默认值
    options := &clientOptions{
        timeout: 30 * time.Second,
        retry:   3,
        debug:   false,
    }

    // 2. 应用所有选项
    for _, opt := range opts {
        opt(options)  // 每个 opt 都是一个函数，调用它来修改 options
    }

    // 3. 用 options 创建 Client
    return &Client{...}
}

// 调用
client := NewClient(WithTimeout(60*time.Second), WithDebug(true))
```

**为什么 Go 要这么复杂？**

- Go 没有默认参数值
- Go 没有命名参数
- Functional Options 是社区公认的最佳实践

---

## 2. 工程化结构：从底层到顶层的封装

### 2.1 层级架构图

```
┌─────────────────────────────────────────────────────────────┐
│                        main.go                               │
│                     (程序入口/组装器)                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ 组装
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                   internal/api/v1/handler.go                 │
│                      (HTTP 接口层)                            │
│                                                              │
│   职责：接收请求 → 参数校验 → 调用服务 → 返回响应               │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ 调用
                              ▼
┌─────────────────────────────────────────────────────────────┐
│               internal/service/                              │
│             (业务逻辑层/服务层)                                │
│                                                              │
│   ├── classroom.go  - 空教室查询服务                          │
│   └── calendar.go   - 日历/周次服务                           │
│                                                              │
│   职责：业务逻辑处理、数据组装、调用底层工具                     │
└─────────────────────────────────────────────────────────────┘
                              │
                              │ 调用
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                         pkg/                                 │
│                    (可复用工具层)                              │
│                                                              │
│   ├── cas/client.go  - HTTP 客户端（Cookie 管理）             │
│   ├── cas/login.go   - CAS 登录流程                          │
│   ├── auth/crypto.go - 密码加密                              │
│   └── logger/        - 日志系统                              │
│                                                              │
│   职责：与外部系统交互、提供基础设施                            │
└─────────────────────────────────────────────────────────────┘
```

### 2.2 为什么要分层？

```python
# Python - 不分层的写法（反面教材）
@app.route('/query')
def query():
    # 登录逻辑
    session = requests.Session()
    login_page = session.get('http://...')
    salt = parse_salt(login_page)
    encrypted = aes_encrypt(password, salt)
    session.post('http://...', data={...})

    # 查询逻辑
    response = session.post('http://...', data={...})
    classrooms = parse_html(response)

    return jsonify(classrooms)

# 问题：
# 1. 代码难以测试（登录、加密、解析混在一起）
# 2. 代码难以复用（其他接口也要登录怎么办？）
# 3. 代码难以维护（登录流程变了要改很多地方）
```

```python
# Python - 分层的写法（推荐）
# pkg/auth/crypto.py
def encrypt_password(password, salt):
    ...

# pkg/cas/client.py
class CASClient:
    def login(self, username, password):
        ...
    def do(self, request):
        ...

# internal/service/classroom.py
class ClassroomService:
    def __init__(self, client: CASClient):
        self.client = client

    def get_empty_classrooms(self, building, date):
        ...

# internal/api/handler.py
class Handler:
    def __init__(self, service: ClassroomService):
        self.service = service

    def query(self, request):
        # 只做参数校验和调用服务
        ...

# main.py
client = CASClient()
client.login(...)
service = ClassroomService(client)
handler = Handler(service)
app.route('/query')(handler.query)
```

### 2.3 依赖关系与调用方向

```
main.go 知道所有层
    │
    ├── 创建 cas.Client
    ├── 调用 client.Login()
    ├── 创建 service.ClassroomService(client)
    ├── 创建 v1.Handler(service)
    └── 注册路由 router.POST("/query", handler.Query)

handler 只知道 service 层
    │
    └── 调用 service.GetEmptyClassrooms()

service 只知道 pkg 层
    │
    ├── 调用 cas.Client.Do() 发请求
    └── 调用 calendar.GetDateInfo() 获取周次

pkg 不知道上层
    │
    └── 只提供工具函数，被上层调用
```

**关键原则：上层依赖下层，下层不依赖上层**

```python
# Python 等效理解
# pkg/ 相当于 utils/ 或第三方库
# internal/service/ 相当于业务逻辑层
# internal/api/ 相当于 Flask/Django 的 views
# main.go 相当于 app.py 或 manage.py
```

### 2.4 pkg 与 internal 的区别

```
pkg/          - 可以被其他项目导入复用
internal/     - 只能被本项目使用（Go 编译器强制限制）
```

```python
# Python 没有 internal 强制限制，但有约定：
# myproject/
#   _internal/    # 下划线前缀表示私有（约定）
#   utils/        # 公开工具
```

---

## 3. Context 上下文详解

### 3.1 Context 是什么？

Context 是 Go 中用于**跨函数传递请求级别数据**的机制，主要用于：

1. **超时控制**：设定操作的最大执行时间
2. **取消信号**：允许取消正在进行的操作
3. **传递值**：在调用链中传递请求相关的数据

```python
# Python 没有内置 Context，但可以这样理解：

# 1. 超时控制 - 类似 requests 的 timeout
requests.get(url, timeout=30)

# 2. 取消信号 - 类似 asyncio 的 cancel
task = asyncio.create_task(some_operation())
task.cancel()

# 3. 传递值 - 类似 Flask 的 g 对象或 threading.local
from flask import g
g.user_id = 123  # 在请求处理过程中传递数据
```

### 3.2 Context 的创建与传递

```go
// 1. 创建根 Context
ctx := context.Background()  // 空的根 Context

// 2. 添加超时限制
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()  // 重要：函数结束时释放资源

// 3. 传递给下层函数
result, err := someFunction(ctx, params)
```

```python
# Python 等效（伪代码）
class Context:
    def __init__(self, timeout=None, parent=None):
        self.timeout = timeout
        self.cancelled = False
        self.deadline = time.time() + timeout if timeout else None

    def is_done(self):
        if self.cancelled:
            return True
        if self.deadline and time.time() > self.deadline:
            return True
        return False

# 使用
ctx = Context(timeout=30)
result = some_function(ctx, params)
```

### 3.3 Context 在调用链中的传递

```
main.go
    │
    │  ctx, cancel := context.WithTimeout(context.Background(), 1*time.Minute)
    │
    └── client.Login(ctx, username, password)
            │
            │  // Login 方法接收 ctx，并传递给子操作
            │
            ├── c.checkNeedCaptcha(ctx, username)
            │       │
            │       └── http.NewRequestWithContext(ctx, "GET", url, nil)
            │              // HTTP 请求绑定了 ctx
            │              // 如果超时，请求自动取消
            │
            ├── c.fetchLoginParams(ctx, loginPageURL)
            │       │
            │       └── http.NewRequestWithContext(ctx, "GET", url, nil)
            │
            ├── c.submitForm(ctx, ...)
            │       │
            │       └── http.NewRequestWithContext(ctx, "POST", url, body)
            │
            └── c.completeSSO(ctx, ticketURL)
                    │
                    └── http.NewRequestWithContext(ctx, "GET", url, nil)
```

**关键点**：

- 每个函数都接收 `ctx` 作为**第一个参数**（Go 惯例）
- 每个 HTTP 请求都用 `NewRequestWithContext` 绑定 ctx
- 如果 1 分钟超时，**所有子操作都会被取消**

### 3.4 为什么 Context 很重要？

**场景 1：防止资源泄漏**

```go
// 没有 Context 的问题
func Login() error {
    resp, _ := http.Get(url)  // 如果服务器不响应，永远卡住
    // ...
}

// 有 Context 的解决方案
func Login(ctx context.Context) error {
    req, _ := http.NewRequestWithContext(ctx, "GET", url, nil)
    resp, err := client.Do(req)
    if err != nil {
        // ctx 超时会返回 context.DeadlineExceeded 错误
        return err
    }
    // ...
}
```

**场景 2：级联取消**

```go
// 用户点击取消按钮
ctx, cancel := context.WithCancel(context.Background())

// 在另一个 goroutine 中执行登录
go func() {
    err := client.Login(ctx, user, pass)
    if errors.Is(err, context.Canceled) {
        fmt.Println("用户取消了登录")
    }
}()

// 用户点击取消
cancel()  // 这会导致 Login 中的所有操作都收到取消信号
```

```python
# Python 等效（asyncio）
async def login():
    try:
        await some_operation()
    except asyncio.CancelledError:
        print("操作被取消")

task = asyncio.create_task(login())
task.cancel()  # 取消任务
```

### 3.5 Context 的最佳实践

```go
// ✅ 正确：ctx 作为第一个参数
func DoSomething(ctx context.Context, param1 string) error

// ❌ 错误：ctx 不是第一个参数
func DoSomething(param1 string, ctx context.Context) error

// ✅ 正确：总是传递 ctx，不要存储
func (s *Service) Query(ctx context.Context) {
    s.client.Do(ctx, req)  // 传递下去
}

// ❌ 错误：把 ctx 存到结构体中
type Service struct {
    ctx context.Context  // 不要这样做！
}

// ✅ 正确：用 defer 确保 cancel 被调用
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
defer cancel()  // 即使函数正常返回，也要释放资源

// ❌ 错误：忘记调用 cancel
ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
// 没有 defer cancel()  -- 资源泄漏！
```

---

## 4. 调用链路完整追踪

### 4.1 用户查询空教室的完整流程

```
用户请求: POST /api/v1/query
    │
    │  请求体: {"building": "老文史楼", "start_node": "01", "end_node": "02", "date_offset": 0}
    │
    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  Gin 框架路由分发                                                    │
│  r.POST("/api/v1/query", apiHandler.QueryClassrooms)                │
└─────────────────────────────────────────────────────────────────────┘
    │
    ▼
┌─────────────────────────────────────────────────────────────────────┐
│  internal/api/v1/handler.go - QueryClassrooms 方法                  │
│                                                                      │
│  func (h *Handler) QueryClassrooms(c *gin.Context) {                │
│      // 1. 解析请求体                                                │
│      var req model.QueryRequest                                     │
│      c.ShouldBindJSON(&req)  // JSON → 结构体                       │
│                                                                      │
│      // 2. 参数校验                                                  │
│      if req.BuildingName == "" { return error }                     │
│                                                                      │
│      // 3. 调用服务层                                                │
│      resp, err := h.classroomService.GetEmptyClassrooms(req)        │
│      ─────────────────────────────────┬─────────────────────────────│
│                                       │                              │
│      // 4. 返回响应                   │                              │
│      c.JSON(200, resp)                │                              │
│  }                                    │                              │
└───────────────────────────────────────┼─────────────────────────────┘
                                        │
                                        ▼
┌─────────────────────────────────────────────────────────────────────┐
│  internal/service/classroom.go - GetEmptyClassrooms 方法            │
│                                                                      │
│  func (s *ClassroomService) GetEmptyClassrooms(req) (*Response, error) {
│      // 1. 获取日历服务（单例）                                       │
│      cal := GetCalendarService()                                     │
│      ─────────────────────────────────┬─────────────────────────────│
│                                       │                              │
│      // 2. 计算周次和日期             │                              │
│      calInfo, dateStr := cal.GetDateInfo(req.DateOffset)            │
│                                       │                              │
│      // 3. 构建 HTTP 请求参数         │                              │
│      params := url.Values{...}        │                              │
│                                       │                              │
│      // 4. 发送请求                   │                              │
│      resp, err := s.client.Do(httpReq)                              │
│      ─────────────────────────────────┼─────────────────────────────│
│                                       │                              │
│      // 5. 解析 HTML 响应             │                              │
│      classrooms := parseHTML(resp)    │                              │
│                                       │                              │
│      return &ClassroomResponse{...}   │                              │
│  }                                    │                              │
└───────────────────────────────────────┼─────────────────────────────┘
                                        │
        ┌───────────────────────────────┴───────────────────────────┐
        │                                                           │
        ▼                                                           ▼
┌───────────────────────────────────┐   ┌───────────────────────────────────┐
│  internal/service/calendar.go     │   │  pkg/cas/client.go                │
│                                   │   │                                   │
│  GetDateInfo(offset int)          │   │  Do(req *http.Request)            │
│      │                            │   │      │                            │
│      ├── 获取当前周次 baseWeek     │   │      └── 使用带 Cookie 的         │
│      ├── 计算目标周次              │   │          httpClient 发送请求      │
│      ├── 计算星期几               │   │                                   │
│      └── 返回 CalendarInfo        │   │  Cookie 在 Login 时已经设置好     │
│                                   │   │                                   │
└───────────────────────────────────┘   └───────────────────────────────────┘
```

### 4.2 对象是如何被创建和传递的？

```go
// main.go - 程序启动时的组装过程

func main() {
    // ========== 第一步：创建底层工具 ==========

    // 创建 CAS 客户端（带 Cookie 管理）
    client, _ := cas.NewClient(cas.WithTimeout(30 * time.Second))
    // client 内部有:
    //   - httpClient（带 CookieJar）
    //   - options（配置）

    // 登录，获取 Session
    client.Login(ctx, username, password)
    // 登录后，client.httpClient.Jar 中存储了会话 Cookie
    // 后续所有请求都会自动携带这些 Cookie


    // ========== 第二步：创建服务层 ==========

    // 初始化日历服务（单例）
    service.InitCalendarService(client)
    // 内部：calendarInstance = &CalendarService{client: client}
    // 日历服务持有 client 的引用，可以发请求获取周次

    // 创建空教室服务
    classroomService := service.NewClassroomService(client)
    // classroomService 持有 client 的引用


    // ========== 第三步：创建 API 层 ==========

    // 创建 Handler
    apiHandler := v1.NewHandler(classroomService)
    // apiHandler 持有 classroomService 的引用


    // ========== 第四步：注册路由 ==========

    r := gin.Default()
    api := r.Group("/api/v1")
    api.POST("/query", apiHandler.QueryClassrooms)
    // Gin 框架持有 apiHandler.QueryClassrooms 方法的引用


    // ========== 对象引用关系 ==========
    //
    //  Gin Router
    //      │
    //      └──→ apiHandler (Handler)
    //               │
    //               └──→ classroomService (ClassroomService)
    //                        │
    //                        └──→ client (cas.Client)
    //                                 │
    //                                 └──→ httpClient (http.Client)
    //                                          │
    //                                          └──→ CookieJar (存储会话)
    //
    //  calendarInstance (单例)
    //      │
    //      └──→ client (同一个 cas.Client)
}
```

```python
# Python 等效
def main():
    # 第一步：创建底层工具
    client = CASClient(timeout=30)
    client.login(username, password)

    # 第二步：创建服务层（依赖注入）
    CalendarService.init(client)  # 单例
    classroom_service = ClassroomService(client)

    # 第三步：创建 API 层
    handler = Handler(classroom_service)

    # 第四步：注册路由
    app = Flask(__name__)
    app.add_url_rule('/api/v1/query', view_func=handler.query, methods=['POST'])
```

### 4.3 为什么所有服务共享同一个 client？

```
                    ┌─────────────────────┐
                    │    cas.Client       │
                    │  ┌───────────────┐  │
                    │  │  CookieJar    │  │  ← 存储登录后的会话 Cookie
                    │  │  (会话状态)    │  │
                    │  └───────────────┘  │
                    └─────────────────────┘
                              ▲
                              │ 共享引用
          ┌───────────────────┼───────────────────┐
          │                   │                   │
┌─────────────────┐  ┌─────────────────┐  ┌─────────────────┐
│ CalendarService │  │ClassroomService │  │  (其他服务...)   │
└─────────────────┘  └─────────────────┘  └─────────────────┘

所有服务共享同一个 client，因此：
1. 只需要登录一次
2. 所有请求都携带相同的会话 Cookie
3. 会话状态在整个应用中保持一致
```

---

## 总结

### 工程化的核心思想

1. **分层**：每层只关心自己的职责
   - API 层：参数校验、调用服务、返回响应
   - 服务层：业务逻辑
   - 工具层：与外部系统交互

2. **依赖注入**：对象从外部传入，而不是内部创建
   - 便于测试（可以传入 mock 对象）
   - 便于复用（同一个对象可以被多处使用）

3. **单例模式**：全局共享的服务只创建一次
   - 日历服务只需初始化一次
   - 避免重复请求

### Context 的核心作用

1. **超时控制**：防止操作永远阻塞
2. **取消传播**：取消操作可以传递到所有子操作
3. **请求级数据传递**：在调用链中传递请求相关的数据

### Python 开发者的建议

- Go 没有 `try/except`，习惯 `if err != nil` 的写法
- Go 没有类继承，用组合和接口代替
- Go 没有默认参数，用 Functional Options 模式
- Go 没有内置 Context，但要把它当作必备工具
- Go 的首字母大小写决定公开/私有，比 Python 的 `_` 约定更严格
