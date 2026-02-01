package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// LogRotator 实现了 io.Writer 接口，用于处理日志文件的自动轮转
type LogRotator struct {
	mu        sync.Mutex
	dir       string
	prefix    string
	ext       string
	file      *os.File
	size      int64
	maxSize   int64 // 字节
	startTime time.Time
	seq       int
}

// NewLogRotator 创建一个新的 LogRotator
// dir: 日志存储目录
// maxSizeMB: 单个日志文件的最大大小（MB），0 表示不限制
func NewLogRotator(dir string, maxSizeMB int) *LogRotator {
	if err := os.MkdirAll(dir, 0755); err != nil {
		fmt.Fprintf(os.Stderr, "创建日志目录失败：%v\n", err)
	}
	return &LogRotator{
		dir:       dir,
		prefix:    "app",
		ext:       ".log",
		maxSize:   int64(maxSizeMB) * 1024 * 1024,
		startTime: time.Now(),
		seq:       0,
	}
}

func (r *LogRotator) openNewFile() error {
	// 关闭现有文件（如果存在）
	if r.file != nil {
		r.file.Close()
	}

	// 格式：app-YYYY-MM-DD_HH-mm-ss[_seq].log
	timestamp := r.startTime.Format("2006-01-02_15-04-05")
	var filename string
	if r.seq == 0 {
		filename = fmt.Sprintf("%s-%s%s", r.prefix, timestamp, r.ext)
	} else {
		filename = fmt.Sprintf("%s-%s_%d%s", r.prefix, timestamp, r.seq, r.ext)
	}

	path := filepath.Join(r.dir, filename)

	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return err
	}

	r.file = f
	r.size = 0

	// 如果是以追加模式打开现有文件（例如同一秒内重启），检查文件大小
	info, err := f.Stat()
	if err == nil {
		r.size = info.Size()
	}

	return nil
}

func (r *LogRotator) Write(p []byte) (n int, err error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 首次写入时打开文件
	if r.file == nil {
		if err := r.openNewFile(); err != nil {
			return 0, err
		}
	}

	// 检查是否需要轮转
	if r.maxSize > 0 && r.size+int64(len(p)) > r.maxSize {
		r.seq++
		if err := r.openNewFile(); err != nil {
			return 0, err
		}
	}

	n, err = r.file.Write(p)
	r.size += int64(n)
	return n, err
}

func (r *LogRotator) Close() error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.file != nil {
		return r.file.Close()
	}
	return nil
}
