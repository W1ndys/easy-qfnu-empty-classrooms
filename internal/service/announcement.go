package service

import (
	"database/sql"
	"fmt"
	"time"

	"github.com/W1ndys/easy-qfnu-kjs/internal/model"
	"github.com/W1ndys/easy-qfnu-kjs/pkg/logger"
)

// AnnouncementService 管理 PostgreSQL 中的公告数据。
type AnnouncementService struct {
	db *sql.DB
}

func NewAnnouncementService(db *sql.DB) *AnnouncementService {
	logger.Info("公告服务已初始化")
	return &AnnouncementService{db: db}
}

// List 获取所有公告（按创建时间倒序）。
func (s *AnnouncementService) List() ([]model.Announcement, error) {
	rows, err := s.db.Query(`
		SELECT id, title, content, important, created_at, updated_at
		FROM announcements ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, fmt.Errorf("查询公告列表失败: %w", err)
	}
	defer rows.Close()

	list := make([]model.Announcement, 0)
	for rows.Next() {
		var announcement model.Announcement
		if err := rows.Scan(
			&announcement.ID,
			&announcement.Title,
			&announcement.Content,
			&announcement.Important,
			&announcement.CreatedAt,
			&announcement.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("扫描公告数据失败: %w", err)
		}
		normalizeAnnouncementTimes(&announcement)
		list = append(list, announcement)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("遍历公告结果失败: %w", err)
	}
	return list, nil
}

// GetByID 根据 ID 获取单条公告。
func (s *AnnouncementService) GetByID(id int64) (*model.Announcement, error) {
	var announcement model.Announcement
	err := s.db.QueryRow(`
		SELECT id, title, content, important, created_at, updated_at
		FROM announcements WHERE id = $1
	`, id).Scan(
		&announcement.ID,
		&announcement.Title,
		&announcement.Content,
		&announcement.Important,
		&announcement.CreatedAt,
		&announcement.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("查询公告失败: %w", err)
	}
	normalizeAnnouncementTimes(&announcement)
	return &announcement, nil
}

// Create 创建公告，所有时间以 UTC 写入 PostgreSQL TIMESTAMPTZ。
func (s *AnnouncementService) Create(req model.CreateAnnouncementRequest) (*model.Announcement, error) {
	now := time.Now().UTC()
	var id int64
	if err := s.db.QueryRow(`
		INSERT INTO announcements (title, content, important, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $4)
		RETURNING id
	`, req.Title, req.Content, req.Important, now).Scan(&id); err != nil {
		return nil, fmt.Errorf("创建公告失败: %w", err)
	}
	return s.GetByID(id)
}

// Update 更新公告。
func (s *AnnouncementService) Update(id int64, req model.UpdateAnnouncementRequest) (*model.Announcement, error) {
	result, err := s.db.Exec(`
		UPDATE announcements
		SET title = $1, content = $2, important = $3, updated_at = $4
		WHERE id = $5
	`, req.Title, req.Content, req.Important, time.Now().UTC(), id)
	if err != nil {
		return nil, fmt.Errorf("更新公告失败: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return nil, fmt.Errorf("获取影响行数失败: %w", err)
	}
	if affected == 0 {
		return nil, nil
	}
	return s.GetByID(id)
}

// Delete 删除公告。
func (s *AnnouncementService) Delete(id int64) error {
	result, err := s.db.Exec(`DELETE FROM announcements WHERE id = $1`, id)
	if err != nil {
		return fmt.Errorf("删除公告失败: %w", err)
	}
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("获取影响行数失败: %w", err)
	}
	if affected == 0 {
		return fmt.Errorf("公告不存在")
	}
	return nil
}

// ListPublic 获取前台展示用的公告列表。
func (s *AnnouncementService) ListPublic() ([]model.AnnouncementPublic, error) {
	announcements, err := s.List()
	if err != nil {
		return nil, err
	}

	list := make([]model.AnnouncementPublic, 0, len(announcements))
	for _, announcement := range announcements {
		list = append(list, model.AnnouncementPublic{
			ID:        announcement.ID,
			Title:     announcement.Title,
			Content:   announcement.Content,
			Important: announcement.Important,
			CreatedAt: announcement.CreatedAt,
		})
	}
	return list, nil
}

func normalizeAnnouncementTimes(announcement *model.Announcement) {
	announcement.CreatedAt = announcement.CreatedAt.UTC()
	announcement.UpdatedAt = announcement.UpdatedAt.UTC()
}
