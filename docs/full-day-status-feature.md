# 教室全天状态查询功能开发文档

## 1. 功能概述

### 1.1 目标
新增一个功能模块，用于查询指定教学楼在指定日期一整天（所有节次）的教室使用状态，并通过不同颜色直观展示各教室在不同节次的占用情况。

### 1.2 功能调整
- **首页改造**：将现有首页改为功能导航入口，提供两个功能选项
  - 功能一：空教室查询（现有功能）
  - 功能二：教室全天状态查询（新增功能）
- **新增页面**：教室全天状态查询页面

### 1.3 教室状态定义

| ID | 状态码 | 状态名称 | 显示颜色 |
|----|--------|----------|----------|
| 1 | ◆ | 正常上课 | 红色 `#EF4444` |
| 2 | Ｊ | 借用 | 橙色 `#F97316` |
| 3 | Ｘ | 锁定 | 灰色 `#6B7280` |
| 4 | Κ | 考试 | 紫色 `#8B5CF6` |
| 5 | 空闲 | 空闲 | 绿色 `#10B981` |
| 6 | Ｇ | 固定调课 | 蓝色 `#3B82F6` |
| 7 | Ｌ | 临时调课 | 青色 `#06B6D4` |
| 8 | 完全空闲 | 完全空闲 | 深绿色 `#059669` |
| 9 | M | 跨模式占用 | 粉色 `#EC4899` |

---

## 2. 后端设计

### 2.1 数据模型

在 `internal/model/types.go` 中新增以下类型：

```go
// FullDayQueryRequest 全天状态查询请求
type FullDayQueryRequest struct {
	BuildingName string `json:"building"`      // 教学楼名称 (如 "老文史楼")
	DateOffset   int    `json:"date_offset"`   // 日期偏移 (0=今天, 1=明天...)
}

// ClassroomStatus 单个教室在单个节次的状态
type ClassroomStatus struct {
	RoomName string `json:"room_name"` // 教室名称 (如 "老文史楼101")
	StatusID int    `json:"status_id"` // 状态ID (1-9)
	StatusCode string `json:"status_code"` // 状态码 (如 "◆", "空闲")
}

// NodeInfo 节次信息
type NodeInfo struct {
	NodeIndex int    `json:"node_index"` // 节次索引 (1-11)
	NodeName  string `json:"node_name"`  // 节次名称 (如 "第1节")
}

// RoomStatus 单个教室在单个节次的状态
type RoomStatus struct {
	NodeIndex  int    `json:"node_index"`  // 节次索引
	StatusID   int    `json:"status_id"`   // 状态ID (1-9)
	StatusCode string `json:"status_code"` // 状态码 (如 "◆", "空闲")
}

// ClassroomFullStatus 单个教室的全天状态
type ClassroomFullStatus struct {
	RoomName string       `json:"room_name"` // 教室名称 (如 "老文史楼101")
	Status   []RoomStatus `json:"status"`    // 各节次状态列表
}

// FullDayStatusResponse 全天状态查询响应
type FullDayStatusResponse struct {
	Date       string                `json:"date"`        // 查询日期 (YYYY-MM-DD)
	Week       int                   `json:"week"`        // 教学周
	DayOfWeek  int                   `json:"day_of_week"` // 星期几 (1-7)
	Building   string                `json:"building"`    // 教学楼名称
	NodeList   []NodeInfo            `json:"node_list"`   // 节次列表（用于前端表头）
	Classrooms []ClassroomFullStatus `json:"classrooms"`  // 各教室全天状态列表
}
```

### 2.2 服务层实现

在 `internal/service/classroom.go` 中新增方法：

```go
// GetFullDayStatus 获取指定教学楼一整天的教室状态
func (s *ClassroomService) GetFullDayStatus(req model.FullDayQueryRequest) (*model.FullDayStatusResponse, error) {
	cal := GetCalendarService()
	if cal == nil {
		return nil, fmt.Errorf("日历服务未初始化")
	}

	// 1. 获取日期和周次信息
	calInfo, dateStr := cal.GetDateInfo(req.DateOffset)

	// 2. 一次查询全天所有节次（jc 和 jc2 置空）
	nodeList, classrooms, err := s.queryFullDay(req.BuildingName, calInfo)
	if err != nil {
		return nil, fmt.Errorf("查询全天状态失败：%w", err)
	}

	weekInt, _ := strconv.Atoi(calInfo.Zc)
	dayInt, _ := strconv.Atoi(calInfo.Xq)

	return &model.FullDayStatusResponse{
		Date:       dateStr,
		Week:       weekInt,
		DayOfWeek:  dayInt,
		Building:   req.BuildingName,
		NodeList:   nodeList,
		Classrooms: classrooms,
	}, nil
}

// queryFullDay 查询全天教室状态
// 关键：jc 和 jc2 置空，同时不设置 jszt 参数，获取全天所有状态
func (s *ClassroomService) queryFullDay(building string, calInfo model.CalendarInfo) ([]model.NodeInfo, []model.ClassroomFullStatus, error) {
	apiURL := "http://zhjw.qfnu.edu.cn/jsxsd/kbxx/jsjy_query2"

	params := url.Values{}
	params.Set("typewhere", "jszq")
	params.Set("xnxqh", calInfo.Xnxqh)
	params.Set("jsmc_mh", building)
	params.Set("bjfh", "=")
	// 关键：不设置 jszt 参数，查询所有状态（不只是空闲）
	params.Set("zc", calInfo.Zc)
	params.Set("zc2", calInfo.Zc)
	params.Set("xq", calInfo.Xq)
	params.Set("xq2", calInfo.Xq)
	// 关键：jc 和 jc2 置空，查询全天所有节次
	// 不设置 jc 和 jc2 参数

	httpReq, err := http.NewRequest("POST", apiURL, strings.NewReader(params.Encode()))
	if err != nil {
		return nil, nil, err
	}
	httpReq.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	httpReq.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := s.client.Do(httpReq)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		return nil, nil, err
	}

	return parseFullDayStatusFromHTML(doc)
}

// parseFullDayStatusFromHTML 从HTML中解析全天教室状态
// 返回数据结构：按教室分组的节次状态列表（教室-节次-状态）
func parseFullDayStatusFromHTML(doc *goquery.Document) ([]model.NodeInfo, []model.ClassroomFullStatus, error) {
	// 解析表格结构：
	// thead 中包含节次信息（第1节、第2节...）
	// tbody 中每行代表一个教室，每列代表该教室在对应节次的状态
	
	var nodeList []model.NodeInfo
	classroomMap := make(map[string]*model.ClassroomFullStatus) // 临时map，key为教室名
	
	// 先解析表头获取节次信息
	headRow := doc.Find("table#dataList thead tr").First()
	nodeColMap := make(map[int]int) // 列索引 -> 节次索引的映射
	
	headRow.Find("th").Each(func(colIdx int, th *goquery.Selection) {
		text := strings.TrimSpace(th.Text())
		// 跳过"星期"列或其他非节次列
		if text == "星期" || text == "" || colIdx == 0 {
			return
		}
		
		nodeIdx := len(nodeList) // 当前节次索引
		nodeColMap[colIdx] = nodeIdx
		
		nodeList = append(nodeList, model.NodeInfo{
			NodeIndex: nodeIdx + 1, // 节次从1开始
			NodeName:  text,
		})
	})
	
	// 解析 tbody 中的教室状态
	doc.Find("table#dataList tbody tr").Each(func(rowIdx int, tr *goquery.Selection) {
		// 获取教室名称（第一列）
		firstTd := tr.Find("td").First()
		text := strings.TrimSpace(firstTd.Text())
		
		roomName := ""
		idx := strings.Index(text, "(")
		if idx > 0 {
			roomName = strings.TrimSpace(text[:idx])
		}
		if roomName == "" {
			return
		}
		
		// 初始化该教室的状态列表
		if _, exists := classroomMap[roomName]; !exists {
			classroomMap[roomName] = &model.ClassroomFullStatus{
				RoomName: roomName,
				Status:   make([]model.RoomStatus, len(nodeList)),
			}
		}
		
		// 遍历每个节次列
		tr.Find("td").Each(func(colIdx int, td *goquery.Selection) {
			if colIdx == 0 {
				return // 跳过第一列（教室名）
			}
			
			nodeIdx, ok := nodeColMap[colIdx]
			if !ok {
				return // 非节次列
			}
			
			statusCode := strings.TrimSpace(td.Text())
			if statusCode == "" {
				statusCode = "空闲" // 空单元格表示空闲
			}
			
			statusID := mapStatusCodeToID(statusCode)
			
			classroomMap[roomName].Status[nodeIdx] = model.RoomStatus{
				NodeIndex:  nodeIdx + 1, // 节次从1开始
				StatusID:   statusID,
				StatusCode: statusCode,
			}
		})
	})
	
	// 将 map 转换为切片
	var classrooms []model.ClassroomFullStatus
	for _, cs := range classroomMap {
		classrooms = append(classrooms, *cs)
	}
	
	// 按教室名称排序
	sort.Slice(classrooms, func(i, j int) bool {
		return classrooms[i].RoomName < classrooms[j].RoomName
	})
	
	return nodeList, classrooms, nil
}

// 需要添加的 import
import "sort"

// mapStatusCodeToID 将状态码映射到ID
func mapStatusCodeToID(code string) int {
	switch code {
	case "◆":
		return 1
	case "Ｊ":
		return 2
	case "Ｘ":
		return 3
	case "Κ":
		return 4
	case "空闲":
		return 5
	case "Ｇ":
		return 6
	case "Ｌ":
		return 7
	case "完全空闲":
		return 8
	case "M":
		return 9
	default:
		return 5 // 默认空闲
	}
}
```

### 2.3 API 接口层

在 `internal/api/v1/handler.go` 中新增接口：

```go
// QueryFullDayStatus 查询全天教室状态
func (h *Handler) QueryFullDayStatus(c *gin.Context) {
	var req model.FullDayQueryRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式错误"})
		return
	}

	if req.BuildingName == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "请输入教学楼名称"})
		return
	}

	resp, err := h.classroomService.GetFullDayStatus(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, resp)
}
```

### 2.4 路由配置

在 `main.go` 中新增路由：

```go
api := r.Group("/api/v1")
{
	api.GET("/status", apiHandler.GetStatus)
	api.POST("/query", apiHandler.QueryClassrooms)
	api.POST("/query-full-day", apiHandler.QueryFullDayStatus) // 新增
}
```

---

## 3. 前端设计

### 3.1 首页改造 (index.html)

将现有查询表单改为功能导航入口：

```html
<!-- Main Content -->
<main class="px-4 py-6 space-y-6 max-w-xl mx-auto">
    
    <!-- Header Section -->
    <div class="text-center py-4">
        <h2 class="text-2xl font-bold text-[#885021]">QFNU 教室查询系统</h2>
        <p class="text-gray-500 mt-2">曲阜师范大学教室资源查询平台</p>
    </div>

    <!-- Feature Cards -->
    <div class="space-y-4">
        <!-- Feature 1: Empty Classroom Query -->
        <a href="/empty-classroom.html" 
           class="block bg-white rounded-2xl p-6 shadow-sm border border-gray-100 hover:shadow-md transition-all active:scale-95">
            <div class="flex items-center space-x-4">
                <div class="w-14 h-14 bg-green-100 rounded-xl flex items-center justify-center flex-shrink-0">
                    <svg class="w-7 h-7 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" 
                              d="M21 21l-6-6m2-5a7 7 0 11-14 0 7 7 0 0114 0z"/>
                    </svg>
                </div>
                <div class="flex-1">
                    <h3 class="text-lg font-semibold text-gray-800">空教室查询</h3>
                    <p class="text-sm text-gray-500 mt-1">查询指定时间段内的空闲教室</p>
                </div>
                <svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
                </svg>
            </div>
        </a>

        <!-- Feature 2: Full Day Status -->
        <a href="/full-day-status.html" 
           class="block bg-white rounded-2xl p-6 shadow-sm border border-gray-100 hover:shadow-md transition-all active:scale-95">
            <div class="flex items-center space-x-4">
                <div class="w-14 h-14 bg-[#885021]/10 rounded-xl flex items-center justify-center flex-shrink-0">
                    <svg class="w-7 h-7 text-[#885021]" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                        <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" 
                              d="M9 17V7m0 10a2 2 0 01-2 2H5a2 2 0 01-2-2V7a2 2 0 012-2h2a2 2 0 012 2m0 10a2 2 0 002 2h2a2 2 0 002-2M9 7a2 2 0 012-2h2a2 2 0 012 2m0 10V7m0 10a2 2 0 002 2h2a2 2 0 002-2V7a2 2 0 00-2-2h-2a2 2 0 00-2 2"/>
                    </svg>
                </div>
                <div class="flex-1">
                    <h3 class="text-lg font-semibold text-gray-800">教室全天状态</h3>
                    <p class="text-sm text-gray-500 mt-1">查看指定教学楼全天各节次的占用情况</p>
                </div>
                <svg class="w-5 h-5 text-gray-400" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
                </svg>
            </div>
        </a>
    </div>

    <!-- Footer -->
    <footer class="text-center text-gray-400 text-xs py-6 mt-8">
        微信公众号「卷卷爱吃曲奇饼干」提供技术支持
    </footer>
</main>
```

### 3.2 空教室查询页面 (empty-classroom.html)

将现有 `index.html` 的内容复制到 `web/empty-classroom.html`，并添加返回按钮：

```html
<!-- Header -->
<header class="bg-white/80 backdrop-blur-md sticky top-0 z-50 border-b border-gray-200 px-4">
    <div class="h-14 flex items-center justify-between">
        <a href="/" class="text-gray-500 hover:text-gray-700">
            <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"/>
            </svg>
        </a>
        <h1 class="text-lg font-semibold">空教室查询</h1>
        <div class="w-6"></div>
    </div>
</header>
```

### 3.3 全天状态查询页面 (full-day-status.html)

新建页面，核心功能：

```html
<!DOCTYPE html>
<html lang="zh-CN">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0, maximum-scale=1.0, user-scalable=no">
    <title>教室全天状态 - QFNU</title>
    <link href="/static/css/style.css" rel="stylesheet">
    <script defer src="https://cdn.jsdelivr.net/npm/alpinejs@3.x.x/dist/cdn.min.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/axios/dist/axios.min.js"></script>
    <style>
        /* 状态颜色定义 */
        .status-1 { background-color: #FEE2E2; color: #DC2626; } /* 正常上课 - 红 */
        .status-2 { background-color: #FFEDD5; color: #EA580C; } /* 借用 - 橙 */
        .status-3 { background-color: #F3F4F6; color: #4B5563; } /* 锁定 - 灰 */
        .status-4 { background-color: #EDE9FE; color: #7C3AED; } /* 考试 - 紫 */
        .status-5 { background-color: #D1FAE5; color: #059669; } /* 空闲 - 绿 */
        .status-6 { background-color: #DBEAFE; color: #2563EB; } /* 固定调课 - 蓝 */
        .status-7 { background-color: #CFFAFE; color: #0891B2; } /* 临时调课 - 青 */
        .status-8 { background-color: #059669; color: #FFFFFF; } /* 完全空闲 - 深绿 */
        .status-9 { background-color: #FCE7F3; color: #DB2777; } /* 跨模式 - 粉 */
        
        /* 表格横向滚动 */
        .table-container {
            overflow-x: auto;
            -webkit-overflow-scrolling: touch;
        }
        
        /* 固定首列 */
        .sticky-col {
            position: sticky;
            left: 0;
            background: white;
            z-index: 10;
            box-shadow: 2px 0 4px rgba(0,0,0,0.05);
        }
    </style>
</head>
<body class="bg-gray-50 font-sans antialiased" x-data="fullDayApp()">

    <!-- Header -->
    <header class="bg-white/80 backdrop-blur-md sticky top-0 z-50 border-b border-gray-200 px-4">
        <div class="h-14 flex items-center justify-between">
            <a href="/" class="text-gray-500 hover:text-gray-700">
                <svg class="w-6 h-6" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                    <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M15 19l-7-7 7-7"/>
                </svg>
            </a>
            <h1 class="text-lg font-semibold">教室全天状态</h1>
            <div class="w-6"></div>
        </div>
    </header>

    <main class="px-4 py-4 max-w-5xl mx-auto space-y-4">
        
        <!-- Search Form -->
        <div class="bg-white rounded-2xl p-4 shadow-sm space-y-4">
            <!-- Building Input -->
            <div>
                <label class="block text-sm font-medium text-gray-500 mb-1.5 ml-1">教学楼</label>
                <input type="text" x-model="form.building"
                    class="w-full bg-gray-100 rounded-xl py-3 px-4 text-[15px] focus:outline-none focus:ring-2 focus:ring-[#885021]/20"
                    placeholder="例如：老文史楼">
            </div>

            <!-- Date Selection -->
            <div>
                <label class="block text-sm font-medium text-gray-500 mb-1.5 ml-1">日期</label>
                <div class="bg-gray-100 p-1 rounded-xl flex">
                    <template x-for="(label, idx) in ['今天', '明天', '后天']" :key="idx">
                        <button @click="form.offset = idx"
                            :class="form.offset === idx ? 'bg-white text-black shadow-sm' : 'text-gray-500'"
                            class="flex-1 py-2 text-[13px] font-medium rounded-lg transition-all"
                            x-text="label"></button>
                    </template>
                </div>
            </div>

            <!-- Query Button -->
            <button @click="search" :disabled="loading || !form.building"
                class="w-full bg-[#885021] text-white font-semibold py-3.5 rounded-xl disabled:opacity-70 flex items-center justify-center">
                <span x-show="!loading">查询全天状态</span>
                <span x-show="loading" class="flex items-center">
                    <svg class="animate-spin -ml-1 mr-2 h-5 w-5" fill="none" viewBox="0 0 24 24">
                        <circle class="opacity-25" cx="12" cy="12" r="10" stroke="currentColor" stroke-width="4"></circle>
                        <path class="opacity-75" fill="currentColor" d="M4 12a8 8 0 018-8V0C5.373 0 0 5.373 0 12h4z"></path>
                    </svg>
                    查询中...
                </span>
            </button>
        </div>

        <!-- Legend -->
        <div x-show="hasSearched" class="bg-white rounded-2xl p-4 shadow-sm">
            <h3 class="text-sm font-medium text-gray-700 mb-3">状态图例</h3>
            <div class="grid grid-cols-3 gap-2 text-xs">
                <template x-for="item in legendItems" :key="item.id">
                    <div class="flex items-center space-x-2">
                        <span :class="'status-' + item.id" class="px-2 py-1 rounded font-medium" x-text="item.code"></span>
                        <span class="text-gray-600" x-text="item.name"></span>
                    </div>
                </template>
            </div>
        </div>

        <!-- Results Table -->
        <div x-show="hasSearched && resultData" class="bg-white rounded-2xl shadow-sm overflow-hidden">
            <!-- Info Bar -->
            <div class="p-4 border-b border-gray-100">
                <p class="text-sm text-gray-600">
                    <span x-text="resultData.building"></span> · 
                    <span x-text="resultData.date"></span> · 
                    第<span x-text="resultData.week"></span>周 · 
                    星期<span x-text="resultData.day_of_week"></span>
                </p>
            </div>

            <!-- Status Table -->
            <div class="table-container">
                <table class="w-full text-sm">
                    <thead>
                        <tr class="bg-gray-50">
                            <th class="sticky-col px-3 py-3 text-left font-medium text-gray-700">教室</th>
                            <template x-for="node in resultData.node_list" :key="node.node_index">
                                <th class="px-2 py-3 text-center font-medium text-gray-700 min-w-[60px]"
                                    x-text="node.node_index"></th>
                            </template>
                        </tr>
                    </thead>
                    <tbody>
                        <template x-for="room in resultData.classrooms" :key="room.room_name">
                            <tr class="border-t border-gray-100">
                                <td class="sticky-col px-3 py-3 font-medium text-gray-800 bg-white" x-text="room.room_name"></td>
                                <template x-for="(status, idx) in room.status" :key="idx">
                                    <td class="px-1 py-2 text-center">
                                        <span :class="'status-' + status.status_id"
                                              class="inline-block px-2 py-1 rounded text-xs font-medium min-w-[32px]"
                                              x-text="status.status_code">
                                        </span>
                                    </td>
                                </template>
                            </tr>
                        </template>
                    </tbody>
                </table>
            </div>
        </div>

        <!-- Empty State -->
        <div x-show="hasSearched && !loading && !resultData" class="py-10 text-center text-gray-400">
            <p>暂无数据</p>
        </div>
    </main>

    <script>
        function fullDayApp() {
            return {
                loading: false,
                hasSearched: false,
                resultData: null,
                
                form: {
                    building: '',
                    offset: 0
                },

                legendItems: [
                    { id: 1, code: '◆', name: '正常上课' },
                    { id: 2, code: 'Ｊ', name: '借用' },
                    { id: 3, code: 'Ｘ', name: '锁定' },
                    { id: 4, code: 'Κ', name: '考试' },
                    { id: 5, code: '空闲', name: '空闲' },
                    { id: 6, code: 'Ｇ', name: '固定调课' },
                    { id: 7, code: 'Ｌ', name: '临时调课' },
                    { id: 8, code: '完全空闲', name: '完全空闲' },
                    { id: 9, code: 'M', name: '跨模式' },
                ],

                async search() {
                    if (!this.form.building) return;
                    
                    this.loading = true;
                    this.hasSearched = false;
                    this.resultData = null;

                    try {
                        const res = await axios.post('/api/v1/query-full-day', {
                            building: this.form.building,
                            date_offset: this.form.offset
                        });

                        this.resultData = res.data;
                        this.hasSearched = true;
                    } catch (error) {
                        console.error(error);
                        alert(error.response?.data?.error || '查询失败');
                    } finally {
                        this.loading = false;
                    }
                },


            }
        }
    </script>
</body>
</html>
```

### 3.4 静态资源更新

更新 `web/assets.go`：

```go
//go:embed index.html empty-classroom.html full-day-status.html css images
var StaticFS embed.FS
```

---

## 4. 实现步骤

### Step 1: 更新数据模型
- [ ] 在 `internal/model/types.go` 中添加新类型

### Step 2: 实现服务层
- [ ] 在 `internal/service/classroom.go` 中添加 `GetFullDayStatus` 方法
- [ ] 添加 `queryFullDay` 辅助方法（jc/jc2 置空查询全天）
- [ ] 添加 `parseFullDayStatusFromHTML` 解析方法（解析多节次表格）
- [ ] 添加 `mapStatusCodeToID` 映射方法

### Step 3: 实现 API 接口
- [ ] 在 `internal/api/v1/handler.go` 中添加 `QueryFullDayStatus` 方法

### Step 4: 配置路由
- [ ] 在 `main.go` 中添加 `/api/v1/query-full-day` 路由

### Step 5: 前端开发
- [ ] 将现有 `index.html` 重命名为 `empty-classroom.html`
- [ ] 新建导航首页 `index.html`
- [ ] 新建全天状态页面 `full-day-status.html`
- [ ] 更新 `web/assets.go` 包含新文件

### Step 6: 编译 CSS
```bash
npm run build:css
```

### Step 7: 测试验证
```bash
go run .
```

---

## 5. 注意事项

### 5.1 查询性能
- 全天状态只需 **1 次 HTTP 请求**（jc 和 jc2 置空即可查询全天）
- 响应包含所有节次的数据，需要在后端解析 HTML 表格结构

### 5.2 HTML 解析注意事项
- 教务系统返回的表格结构可能变化
- 建议添加容错处理，某个节次解析失败时继续处理其他节次
- 空单元格表示空闲状态

---

## 6. API 测试示例

### 请求
```http
POST /api/v1/query-full-day
Content-Type: application/json

{
    "building": "老文史楼",
    "date_offset": 0
}
```

### 响应
```json
{
    "date": "2026-02-10",
    "week": 1,
    "day_of_week": 2,
    "building": "老文史楼",
    "node_list": [
        { "node_index": 1, "node_name": "第1节" },
        { "node_index": 2, "node_name": "第2节" },
        { "node_index": 3, "node_name": "第3节" }
    ],
    "classrooms": [
        {
            "room_name": "老文史楼101",
            "status": [
                { "node_index": 1, "status_id": 1, "status_code": "◆" },
                { "node_index": 2, "status_id": 5, "status_code": "空闲" },
                { "node_index": 3, "status_id": 5, "status_code": "空闲" }
            ]
        },
        {
            "room_name": "老文史楼102",
            "status": [
                { "node_index": 1, "status_id": 5, "status_code": "空闲" },
                { "node_index": 2, "status_id": 1, "status_code": "◆" },
                { "node_index": 3, "status_id": 5, "status_code": "空闲" }
            ]
        }
    ]
}
```

---

**文档版本**: v1.0  
**创建时间**: 2026-02-10  
**适用项目**: easy-qfnu-empty-classrooms
