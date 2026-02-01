# 曲阜师范大学空教室查询系统开发文档

## 1. 项目概述

本项目旨在开发一个基于 Web 的空教室查询系统。前端采用轻量级技术栈（Tailwind CSS v4 + Alpine.js），后端采用 Go (Gin) 进行 API 服务和静态资源托管。系统核心功能是连接教务系统，根据用户选择的时间和地点，实时查询空闲教室。

### 技术栈

*   **前端**:
    *   HTML5
    *   Tailwind CSS v4 (通过 input.css 编译)
    *   Alpine.js (轻量级响应式框架)
    *   Axios (HTTP 请求)
*   **后端**:
    *   Go (Golang)
    *   Gin Web Framework (Web 服务)
    *   `go:embed` (静态资源打包)
    *   `goquery` (HTML 解析)

---

## 2. 系统架构与目录结构

采用分层架构设计，确保代码模块化、单一职责。

```text
easy-qfnu-empty-classrooms/
├── cmd/
│   └── server/
│       └── main.go           # 程序入口，负责初始化和启动 Server
├── internal/
│   ├── model/                # 数据模型定义 (Request/Response Structs)
│   ├── service/              # 业务逻辑层
│   │   ├── auth.go           # 封装 CAS 登录逻辑 (调用 pkg/cas)
│   │   ├── calendar.go       # 学期与周次管理 (内存持久化)
│   │   └── classroom.go      # 教室查询逻辑
│   └── api/                  # HTTP 接口层 (Gin Handlers)
│       └── v1/
│           └── handler.go
├── pkg/
│   └── cas/                  # 现有的 CAS 登录基础库
├── web/                      # 前端资源
│   ├── index.html            # 单页应用入口
│   ├── css/
│   │   ├── input.css         # Tailwind 源文件
│   │   └── style.css         # 编译后的 CSS
│   └── js/                   # (可选，逻辑可直接写在 HTML 或单独文件)
├── docs/                     # 文档
├── go.mod
└── go.sum
```

---

## 3. 后端详细设计

### 3.1 数据模型 (internal/model)

定义前后端交互的数据结构。

**Request: 查询参数**
```go
type QueryRequest struct {
    BuildingName string `json:"building"`     // 教学楼名称 (如 "老文史楼")
    StartNode    string `json:"start_node"`   // 起始节次 (如 "01")
    EndNode      string `json:"end_node"`     // 终止节次 (如 "02")
    DateOffset   int    `json:"date_offset"`  // 日期偏移 (0=今天, 1=明天...)
}
```

**Response: 教室列表**
```go
type ClassroomResponse struct {
    Date       string   `json:"date"`        // 查询日期 (YYYY-MM-DD)
    Week       int      `json:"week"`        // 教学周
    DayOfWeek  int      `json:"day_of_week"` // 星期几
    Classrooms []string `json:"classrooms"`  // 空教室列表
}
```

### 3.2 业务服务层 (internal/service)

#### A. 学历服务 (`calendar.go`)
**职责**: 系统启动时自动获取当前学期和周次，并驻留在内存中。

*   **数据结构**:
    ```go
    type CalendarState struct {
        CurrentYearStr string // 学年学期 (e.g., "2025-2026-1")
        BaseDate       time.Time // 基准日期 (用于计算偏移)
        BaseWeek       int       // 基准周次
        mu             sync.RWMutex
    }
    ```
*   **核心方法**:
    *   `InitCalendar()`: 启动时调用，访问 `http://zhjw.qfnu.edu.cn/jsxsd/framework/jsMain_new.jsp?t1=1`，解析 HTML 获取“第X周”。同时记录当前系统时间作为基准。
    *   `GetTargetDateInfo(offset int) (week int, dayOfWeek int, dateStr string)`: 根据偏移量计算目标日期的周次和星期。
        *   逻辑：`TargetTime = Now.AddDate(0, 0, offset)`
        *   根据 TargetTime 与 BaseDate 的差值，推算 Week 的变化。

#### B. 教室服务 (`classroom.go`)
**职责**: 处理查询请求，调用 CAS 客户端与教务系统交互。

*   **核心方法**:
    *   `GetEmptyClassrooms(req model.QueryRequest) ([]string, error)`
    *   **流程**:
        1.  调用 `CalendarService` 获取目标日期的 `xnxqh` (学年学期), `zc` (周次), `xq` (星期)。
        2.  构建 POST 请求参数 (参考 `docs/get-classroom-staus.md`)。
            *   `jszt=8` (完全空闲)
            *   `jsmc_mh` (教学楼名称，需 URL 编码)
        3.  使用 `pkg/cas` 中的 Client 发送请求。
        4.  使用 `goquery` 解析返回的 HTML 表格，提取教室名称。

### 3.3 接口层 (internal/api)

*   **Router**: 使用 Gin。
*   `GET /`: 返回 `web/index.html` (HTML 渲染)。
*   `GET /static/*`: 静态资源服务。
*   `POST /api/v1/query`: 接收 JSON，调用 Service，返回 JSON。

---

## 4. 前端详细设计

### 4.1 开发环境配置
使用 Tailwind CSS v4 CLI 进行实时编译：
```bash
# 安装(如果使用 standalone CLI) 或通过 npm
npm install -D tailwindcss @tailwindcss/cli
# 编译命令
npx @tailwindcss/cli -i ./web/css/input.css -o ./web/css/style.css --watch
```

### 4.2 UI 结构 (web/index.html)

基于 `docs/ui.md` 的设计规范（曲奇棕主题）。

**主要组件**:
1.  **Header**: 标题 "QFNU 空教室查询"。
2.  **Form 区域 (Alpine.js x-data="{ query: { ... } }")**:
    *   **教学楼输入**: `<input type="text">` (绑定 query.building)。
    *   **日期选择**: Segmented Control (分段控制器) 风格。
        *   [今天] [明天] [后天] (绑定 query.date_offset)。
    *   **节次选择**: 下拉框或 Grid 选择。
        *   起始: 01, 03, 05, 07, 09
        *   终止: 02, 04, 06, 08, 10
    *   **查询按钮**: 绑定 `@click="fetchClassrooms"`，加载状态显示 Loading Spinner。
3.  **结果展示区域**:
    *   **Loading 态**: 骨架屏 (Skeleton)。
    *   **Empty 态**: SVG 插画 + "暂无空闲教室"。
    *   **List 态**: Grid 布局展示教室卡片。

### 4.3 交互逻辑 (Alpine.js)

```javascript
document.addEventListener('alpine:init', () => {
    Alpine.data('app', () => ({
        loading: false,
        results: [],
        form: {
            building: '',
            offset: 0,
            start: '01',
            end: '02'
        },
        async search() {
            this.loading = true;
            try {
                const res = await axios.post('/api/v1/query', {
                    building: this.form.building,
                    start_node: this.form.start,
                    end_node: this.form.end,
                    date_offset: this.form.offset
                });
                this.results = res.data.classrooms;
            } catch (e) {
                alert('查询失败');
            } finally {
                this.loading = false;
            }
        }
    }))
})
```

---

## 5. 嵌入与构建

使用 Go 1.16+ 的 embed 功能将前端打入二进制文件。

```go
// cmd/server/main.go

//go:embed web/*
var staticFS embed.FS

func main() {
    // ... 初始化 Gin
    // 配置静态文件路由指向 embed.FS
}
```

## 6. 开发步骤建议

1.  **后端先行**:
    *   完善 `pkg/cas` 确保登录稳定。
    *   实现 `CalendarService`，编写单元测试确保能解析 "第X周"。
    *   实现 `ClassroomService`，测试解析空教室 HTML 表格。
2.  **API 联调**:
    *   启动 Gin Server，使用 Postman 测试 `/api/v1/query`。
3.  **前端实现**:
    *   编写 HTML/CSS，对接 API。
4.  **整合**:
    *   配置 Embed，编译最终产物。

---
**注意**: 在开发过程中，请严格遵循 `docs/ui.md` 中的配色方案（主色 #885021）和交互规范。
