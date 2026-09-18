// Package audit 提供安全审计事件的结构化独立落盘，与运行日志分离。
package audit

import (
	"log/slog"
	"os"
)

// Logger 独立审计通道：JSON 行写入专用文件，每行一个事件。
// 写入失败静默忽略，不影响认证主流程。
type Logger struct {
	file *os.File
	log  *slog.Logger
}

// New 打开审计文件并创建记录器，文件以追加方式写入。
func New(path string) (*Logger, error) {
	file, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600)
	if err != nil {
		return nil, err
	}
	return &Logger{
		file: file,
		log:  slog.New(slog.NewJSONHandler(file, nil)),
	}, nil
}

// Event 记录一条审计事件，event 为事件名（如 state_reject、verify_sign_reject），attrs 为业务字段。
func (a *Logger) Event(event string, attrs ...any) {
	if a == nil {
		return
	}
	a.log.Info(event, attrs...)
}

// Close 关闭审计文件。
func (a *Logger) Close() error {
	if a == nil {
		return nil
	}
	return a.file.Close()
}
