// Package capture 捕获 B 站弹幕服务器下发的原始数据包。
//
// 原始包以帧形式写入磁盘,通过 lumberjack 按大小轮转、保留有限份数并
// gzip 压缩旧文件,磁盘占用有硬上限,不会无限膨胀。
//
// 帧格式:
//
//	[4B magic "BLCP"][8B unixnano 时间戳][4B 包长度][原始包字节]
//
// 带 magic 便于回放工具检测损坏与重新同步。服务器更新数据格式导致解析器
// 失败时,原始字节仍然完整,可配合 cmd/replay 进行逆向分析。
package capture

import (
	"encoding/binary"
	"log/slog"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	magic           = 0x424c4350 // "BLCP"
	frameHeaderSize = 4 + 8 + 4  // magic + 时间戳 + 包长度
)

var (
	mu     sync.Mutex
	writer *lumberjack.Logger
)

// Init 初始化捕获。enabled 为 false 或 path 为空时关闭捕获;
// maxSizeMB 默认 100,maxBackups 默认 5。重复调用会替换现有捕获。
func Init(path string, enabled bool, maxSizeMB, maxBackups int) {
	mu.Lock()
	defer mu.Unlock()
	if writer != nil {
		writer.Close()
		writer = nil
	}
	if !enabled || path == "" {
		return
	}
	if maxSizeMB <= 0 {
		maxSizeMB = 100
	}
	if maxBackups <= 0 {
		maxBackups = 5
	}
	if dir := filepath.Dir(path); dir != "" && dir != "." {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			slog.Warn("创建捕获目录失败,捕获将不可用", "dir", dir, "error", err)
		}
	}
	writer = &lumberjack.Logger{
		Filename:   path,
		MaxSize:    maxSizeMB,
		MaxBackups: maxBackups,
		Compress:   true,
	}
}

// Write 将原始数据包写入捕获文件。捕获未启用或写入失败时静默丢弃,不影响主流程。
func Write(packet []byte) {
	if len(packet) == 0 {
		return
	}
	mu.Lock()
	defer mu.Unlock()
	if writer == nil {
		return
	}
	frame := make([]byte, frameHeaderSize+len(packet))
	binary.BigEndian.PutUint32(frame[0:4], magic)
	binary.BigEndian.PutUint64(frame[4:12], uint64(time.Now().UnixNano()))
	binary.BigEndian.PutUint32(frame[12:16], uint32(len(packet)))
	copy(frame[frameHeaderSize:], packet)
	writer.Write(frame)
}

// Close 关闭捕获文件。程序退出时调用。
func Close() {
	mu.Lock()
	defer mu.Unlock()
	if writer != nil {
		writer.Close()
		writer = nil
	}
}
