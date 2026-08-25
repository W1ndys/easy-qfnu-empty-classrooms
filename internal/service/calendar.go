package service

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/PuerkitoBio/goquery"
	"github.com/W1ndys/easy-qfnu-kjs/internal/model"
	"github.com/W1ndys/easy-qfnu-kjs/pkg/cas"
	"github.com/W1ndys/easy-qfnu-kjs/pkg/logger"
)

type httpDoer interface {
	Do(*http.Request) (*http.Response, error)
}

type CalendarService struct {
	client         httpDoer
	currentYearStr string    // 学年学期 e.g. "2025-2026-1"
	baseTime       time.Time // 获取周次的时间点
	baseWeek       int       // 获取到的当前周次
	totalWeeks     int       // 当前学期总周数
	hasPermission  bool      // 是否有权限访问
	now            func() time.Time
	mu             sync.RWMutex
	refreshMu      sync.Mutex
}

var chinaLocation = time.FixedZone("Asia/Shanghai", 8*60*60)

var ErrDateOutsideCurrentTerm = errors.New("目标日期超出当前学期教学周范围")

func (s *CalendarService) currentTime() time.Time {
	if s.now != nil {
		return s.now().In(chinaLocation)
	}
	return time.Now().In(chinaLocation)
}

var calendarInstance *CalendarService

// GetCalendarService 单例获取
func GetCalendarService() *CalendarService {
	return calendarInstance
}

// InitCalendarService 初始化日历服务
func InitCalendarService(client *cas.Client) error {
	calendarInstance = &CalendarService{
		client: client,
		now:    time.Now,
	}
	return calendarInstance.Refresh()
}

// nextRefreshTime 返回给定时刻之后的下一个北京时间 00:01。
func nextRefreshTime(value time.Time) time.Time {
	local := value.In(chinaLocation)
	next := time.Date(local.Year(), local.Month(), local.Day(), 0, 1, 0, 0, chinaLocation)
	if !next.After(local) {
		next = next.AddDate(0, 0, 1)
	}
	return next
}

// StartDailyRefresh 启动每日北京时间 00:01 自动刷新周次信息的后台任务
func (s *CalendarService) StartDailyRefresh() {
	go func() {
		for {
			now := s.currentTime()
			next := nextRefreshTime(now)
			waitDuration := next.Sub(now)
			logger.Info("周次自动刷新已调度，将在 %s 执行（等待 %v）", next.Format("2006-01-02 15:04:05"), waitDuration)

			time.Sleep(waitDuration)

			logger.Info("正在执行每日周次刷新...")
			if err := s.Refresh(); err != nil {
				logger.Warn("每日周次刷新失败: %v", err)
			} else {
				logger.Info("每日周次刷新成功，当前周次: %d", s.GetBaseWeek())
			}
		}
	}()
}

// Refresh 从教务系统刷新当前周次信息
func (s *CalendarService) Refresh() error {
	s.refreshMu.Lock()
	defer s.refreshMu.Unlock()

	s.mu.RLock()
	currentYearStr := s.currentYearStr
	hasPermission := s.hasPermission
	s.mu.RUnlock()

	// 先在局部变量中完成全部网络请求与解析，最后一次性提交状态，
	// 避免请求中途失败后出现“新学期 + 旧周次”的混合快照。
	termURL := "http://zhjw.qfnu.edu.cn/jsxsd/kbxx/jsjy_query"
	termReq, err := http.NewRequest("GET", termURL, nil)
	if err != nil {
		return err
	}
	termResp, err := s.client.Do(termReq)
	if err == nil {
		bodyBytes, readErr := io.ReadAll(termResp.Body)
		termResp.Body.Close()
		if readErr != nil {
			return fmt.Errorf("读取学期信息失败: %w", readErr)
		}
		bodyString := string(bodyBytes)
		if strings.Contains(bodyString, "非法访问") {
			hasPermission = false
			logger.Warn("警告：该账号无权限访问空教室查询接口 (jsjy_query)，请检查账号权限或登录状态。")
		} else {
			hasPermission = true
		}

		termDoc, parseErr := goquery.NewDocumentFromReader(strings.NewReader(bodyString))
		if parseErr != nil {
			return fmt.Errorf("解析学期信息失败: %w", parseErr)
		}
		termMatch := regexp.MustCompile(`\d{4}-\d{4}-\d`).FindString(termDoc.Text())
		if termMatch != "" {
			currentYearStr = termMatch
		}
	} else {
		logger.Warn("警告：无法查询学期信息：%v", err)
	}

	weekURL := "http://zhjw.qfnu.edu.cn/jsxsd/framework/jsMain_new.jsp?t1=1"
	weekReq, err := http.NewRequest("GET", weekURL, nil)
	if err != nil {
		return err
	}
	weekResp, err := s.client.Do(weekReq)
	if err != nil {
		return fmt.Errorf("fetch calendar failed: %w", err)
	}
	defer weekResp.Body.Close()

	doc, err := goquery.NewDocumentFromReader(weekResp.Body)
	if err != nil {
		return err
	}
	htmlContent, err := doc.Html()
	if err != nil {
		return fmt.Errorf("读取周次响应失败: %w", err)
	}

	baseWeek, totalWeeks, weekOK := parseWeekProgress(htmlContent)
	if !weekOK {
		switch {
		case strings.Contains(htmlContent, "非法访问"):
			baseWeek, totalWeeks, hasPermission = 0, 0, false
			logger.Warn("警告：访问首页周次接口检测到'非法访问'，可能无权限或 Session 过期。")
		case strings.Contains(htmlContent, "不在教学周历内"):
			baseWeek, totalWeeks = 0, 0
			logger.Warn("警告：当前日期不在教学周历内。")
		case !hasPermission:
			baseWeek, totalWeeks = 0, 0
			logger.Warn("注意：因无权限访问，无法解析周次信息，服务将以受限模式运行。")
		default:
			return fmt.Errorf("无法从响应中解析周次信息，内容长度：%d", len(htmlContent))
		}
	}

	if currentYearStr == "" {
		currentYearStr = "2025-2026-1"
	}
	baseTime := s.currentTime()

	s.mu.Lock()
	s.currentYearStr = currentYearStr
	s.hasPermission = hasPermission
	s.baseWeek = baseWeek
	s.totalWeeks = totalWeeks
	s.baseTime = baseTime
	s.mu.Unlock()

	logger.Info("日历已初始化：学期=%s，周次=%d，基准时间=%s", currentYearStr, baseWeek, baseTime.Format("2006-01-02"))
	return nil
}

func parseWeekProgress(content string) (current, total int, ok bool) {
	matches := regexp.MustCompile(`li_showWeek[\s\S]{0,500}?第(\d+)周[\s\S]{0,200}?[[:space:]]*[/／][[:space:]]*(\d+)周`).FindStringSubmatch(content)
	if len(matches) != 3 {
		return 0, 0, false
	}
	current, currentErr := strconv.Atoi(matches[1])
	total, totalErr := strconv.Atoi(matches[2])
	return current, total, currentErr == nil && totalErr == nil && current > 0 && total >= current
}

// IsInTeachingCalendar 检查当前是否在教学周历内
func (s *CalendarService) IsInTeachingCalendar() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.baseWeek > 0
}

// GetBaseWeek 获取当前基准周次
func (s *CalendarService) GetBaseWeek() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.baseWeek
}

// GetCurrentYearStr 获取当前学年学期字符串
func (s *CalendarService) GetCurrentYearStr() string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.currentYearStr
}

// startOfMonday 返回指定时间所在周的北京时间周一零点。
func startOfMonday(value time.Time) time.Time {
	local := value.In(chinaLocation)
	weekday := int(local.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	dayStart := time.Date(local.Year(), local.Month(), local.Day(), 0, 0, 0, 0, chinaLocation)
	return dayStart.AddDate(0, 0, -(weekday - 1))
}

// GetDateInfo 根据偏移量计算目标日期的信息
func (s *CalendarService) GetDateInfo(offset int) (info model.CalendarInfo, dateStr string, err error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	targetDate := s.currentTime().AddDate(0, 0, offset)
	dateStr = targetDate.Format("2006-01-02")

	currentWeek := s.baseWeek
	if currentWeek > 0 && !s.baseTime.IsZero() {
		baseMonday := startOfMonday(s.baseTime)
		targetMonday := startOfMonday(targetDate)
		weeksDiff := int(targetMonday.Sub(baseMonday).Hours()) / (24 * 7)
		currentWeek += weeksDiff
	}
	if s.totalWeeks > 0 && (currentWeek < 1 || currentWeek > s.totalWeeks) {
		return model.CalendarInfo{}, dateStr, ErrDateOutsideCurrentTerm
	}

	targetWeekday := int(targetDate.Weekday())
	if targetWeekday == 0 {
		targetWeekday = 7
	}

	info = model.CalendarInfo{
		Xnxqh: s.currentYearStr,
		Zc:    strconv.Itoa(currentWeek),
		Xq:    strconv.Itoa(targetWeekday),
	}

	return info, dateStr, nil
}

// HasPermission 返回是否有权限访问
func (s *CalendarService) HasPermission() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.hasPermission
}
