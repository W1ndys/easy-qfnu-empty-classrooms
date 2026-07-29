package v1

import (
	"crypto/subtle"
	"net/http"
	"strconv"
	"strings"

	"github.com/W1ndys/easy-qfnu-kjs/internal/model"
	"github.com/W1ndys/easy-qfnu-kjs/internal/service"
	"github.com/W1ndys/easy-qfnu-kjs/pkg/jwt"
	"github.com/gin-gonic/gin"
)

// AdminHandler 管理后台 API 处理器
type AdminHandler struct {
	announcementService  *service.AnnouncementService
	openAPIConfigService *service.OpenAPIConfigService
	jwtManager           *jwt.Manager
	adminUsername        string
	adminPassword        string
}

// NewAdminHandler 创建管理后台处理器
func NewAdminHandler(
	as *service.AnnouncementService,
	oacs *service.OpenAPIConfigService,
	jm *jwt.Manager,
	username, password string,
) *AdminHandler {
	return &AdminHandler{
		announcementService:  as,
		openAPIConfigService: oacs,
		jwtManager:           jm,
		adminUsername:        username,
		adminPassword:        password,
	}
}

// Login 管理员登录
func (h *AdminHandler) Login(c *gin.Context) {
	var req model.AdminLoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式错误"})
		return
	}

	// 使用常量时间比较防止时序攻击
	usernameMatch := subtle.ConstantTimeCompare([]byte(req.Username), []byte(h.adminUsername))
	passwordMatch := subtle.ConstantTimeCompare([]byte(req.Password), []byte(h.adminPassword))
	if usernameMatch != 1 || passwordMatch != 1 {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "用户名或密码错误"})
		return
	}

	token, err := h.jwtManager.Generate(req.Username)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "生成 token 失败"})
		return
	}

	c.JSON(http.StatusOK, model.AdminLoginResponse{Token: token})
}

// ---- 公告管理 CRUD ----

// ListAnnouncements 获取公告列表 (管理后台)
func (h *AdminHandler) ListAnnouncements(c *gin.Context) {
	list, err := h.announcementService.List()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取公告列表失败"})
		return
	}
	c.JSON(http.StatusOK, gin.H{"announcements": list})
}

// CreateAnnouncement 创建公告
func (h *AdminHandler) CreateAnnouncement(c *gin.Context) {
	var req model.CreateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式错误"})
		return
	}

	a, err := h.announcementService.Create(req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "创建公告失败"})
		return
	}

	c.JSON(http.StatusCreated, a)
}

// UpdateAnnouncement 更新公告
func (h *AdminHandler) UpdateAnnouncement(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的公告 ID"})
		return
	}

	var req model.UpdateAnnouncementRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式错误"})
		return
	}

	a, err := h.announcementService.Update(id, req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "更新公告失败"})
		return
	}
	if a == nil {
		c.JSON(http.StatusNotFound, gin.H{"error": "公告不存在"})
		return
	}

	c.JSON(http.StatusOK, a)
}

// DeleteAnnouncement 删除公告
func (h *AdminHandler) DeleteAnnouncement(c *gin.Context) {
	id, err := strconv.ParseInt(c.Param("id"), 10, 64)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "无效的公告 ID"})
		return
	}

	if err := h.announcementService.Delete(id); err != nil {
		c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "删除成功"})
}

// GetOpenAPIConfig 获取开放接口配置，响应不会包含明文 Key。
func (h *AdminHandler) GetOpenAPIConfig(c *gin.Context) {
	if h.openAPIConfigService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "开放接口配置服务未初始化"})
		return
	}
	cfg, err := h.openAPIConfigService.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取开放接口配置失败"})
		return
	}
	c.JSON(http.StatusOK, toOpenAPIConfigResponse(cfg))
}

// UpdateOpenAPIConfig 保存开放接口配置；省略 api_key 时保留现有 Key。
func (h *AdminHandler) UpdateOpenAPIConfig(c *gin.Context) {
	if h.openAPIConfigService == nil {
		c.JSON(http.StatusServiceUnavailable, gin.H{"error": "开放接口配置服务未初始化"})
		return
	}
	var req model.UpdateOpenAPIConfigRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "参数格式错误"})
		return
	}
	current, err := h.openAPIConfigService.Get()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取开放接口配置失败"})
		return
	}
	key := current.APIKey
	if req.APIKey != nil {
		key = strings.TrimSpace(*req.APIKey)
	}
	if req.Enabled && key == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "启用开放接口时必须配置授权 Key"})
		return
	}

	cfg, err := h.openAPIConfigService.Update(req.Enabled, key)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "保存开放接口配置失败"})
		return
	}
	c.JSON(http.StatusOK, toOpenAPIConfigResponse(cfg))
}

func toOpenAPIConfigResponse(cfg *model.OpenAPIConfig) model.OpenAPIConfigResponse {
	return model.OpenAPIConfigResponse{
		Enabled:   cfg.Enabled,
		HasAPIKey: cfg.APIKey != "",
		UpdatedAt: cfg.UpdatedAt,
	}
}

// ---- 前台公开接口 ----

// GetPublicAnnouncements 获取前台公告列表 (无需认证)
func (h *AdminHandler) GetPublicAnnouncements(c *gin.Context) {
	list, err := h.announcementService.ListPublic()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "获取公告失败"})
		return
	}
	c.JSON(http.StatusOK, model.AnnouncementListResponse{Announcements: list})
}
