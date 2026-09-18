package audit

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestEventWritesJSONLines(t *testing.T) {
	path := filepath.Join(t.TempDir(), "audit.log")
	a, err := New(path)
	if err != nil {
		t.Fatalf("创建审计记录器失败: %v", err)
	}

	a.Event("state_reject", "app", "oa", "remote", "1.2.3.4")
	a.Event("verify_ok", "app", "oa", "userid", "zhangsan")
	if err := a.Close(); err != nil {
		t.Fatalf("关闭审计文件失败: %v", err)
	}

	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("打开审计文件失败: %v", err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	var lines []map[string]any
	for scanner.Scan() {
		var line map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			t.Fatalf("审计行不是合法 JSON: %v", err)
		}
		lines = append(lines, line)
	}
	if len(lines) != 2 {
		t.Fatalf("审计行数期望 2，实际 %d", len(lines))
	}
	if lines[0]["msg"] != "state_reject" || lines[0]["app"] != "oa" || lines[0]["remote"] != "1.2.3.4" {
		t.Errorf("首行事件字段不符: %v", lines[0])
	}
	if lines[1]["msg"] != "verify_ok" || lines[1]["userid"] != "zhangsan" {
		t.Errorf("次行事件字段不符: %v", lines[1])
	}
}

func TestNilLoggerSafe(t *testing.T) {
	var a *Logger
	a.Event("verify_ok")
	if err := a.Close(); err != nil {
		t.Errorf("nil 记录器 Close 不应报错: %v", err)
	}
}
