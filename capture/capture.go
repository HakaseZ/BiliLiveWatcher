// Package capture 捕获 B 站弹幕服务器下发的原始数据包。
//
// 原始包以帧形式异步写入磁盘(有界队列,队列满时丢弃捕获帧并计数,不阻塞
// WebSocket 读路径),通过 lumberjack 按大小轮转、保留有限份数并 gzip 压缩
// 旧文件,磁盘占用有硬上限,不会无限膨胀。
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
	"sync/atomic"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

const (
	magic           = 0x424c4350 // "BLCP"
	frameHeaderSize = 4 + 8 + 4  // magic + 时间戳 + 包长度
	queueSize       = 1024       // 异步写入队列容量,满则丢弃捕获帧
)

// mu 保护 queue/writer 生命周期:Write 持有读锁发送(与 close 互斥),
// Init/Close 持有写锁分离并关闭队列。writeLoop 不触碰该锁,避免死锁。
var (
	mu          sync.RWMutex
	writer      *lumberjack.Logger
	queue       chan []byte
	writeDone   chan struct{}
	dropped     atomic.Uint64 // 队列满被丢弃的捕获帧数
	writeErrors atomic.Uint64 // 落盘失败的捕获帧数
)

// Init 初始化捕获。enabled 为 false 或 path 为空时关闭捕获;
// maxSizeMB 默认 100,maxBackups 默认 5。重复调用会替换现有捕获。
func Init(path string, enabled bool, maxSizeMB, maxBackups int) {
	mu.Lock()
	defer mu.Unlock()
	stopWriteLoopLocked()
	if writer != nil {
		if err := writer.Close(); err != nil {
			slog.Warn("关闭捕获文件失败", "error", err)
		}
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
	queue = make(chan []byte, queueSize)
	writeDone = make(chan struct{})
	// writeLoop 闭包捕获 writer,不触碰 mu,等待期间不会与持锁的 Init/Close 死锁
	go func(w *lumberjack.Logger) {
		defer close(writeDone)
		for frame := range queue {
			if _, err := w.Write(frame); err != nil {
				writeErrors.Add(1)
				if writeErrors.Load() == 1 {
					slog.Warn("捕获写入失败", "error", err)
				}
			}
		}
	}(writer)
}

// stopWriteLoopLocked 关闭当前队列并等待后台写入协程退出。调用方必须持有 mu 写锁。
func stopWriteLoopLocked() {
	if queue != nil {
		close(queue)
		<-writeDone
		queue = nil
		writeDone = nil
	}
}

// Write 将原始数据包入队写入捕获文件。捕获未启用时为空操作;
// 队列满时丢弃该帧并计数,不阻塞调用方。
func Write(packet []byte) {
	if len(packet) == 0 {
		return
	}
	frame := make([]byte, frameHeaderSize+len(packet))
	binary.BigEndian.PutUint32(frame[0:4], magic)
	binary.BigEndian.PutUint64(frame[4:12], uint64(time.Now().UnixNano()))
	binary.BigEndian.PutUint32(frame[12:16], uint32(len(packet)))
	copy(frame[frameHeaderSize:], packet)

	mu.RLock()
	defer mu.RUnlock()
	if queue == nil {
		return
	}
	select {
	case queue <- frame:
	default:
		dropped.Add(1)
	}
}

// Dropped 返回因队列满被丢弃的捕获帧数(供监控排查)
func Dropped() uint64 {
	return dropped.Load()
}

// WriteErrors 返回落盘失败的捕获帧数(供监控排查)
func WriteErrors() uint64 {
	return writeErrors.Load()
}

// Close 关闭捕获文件并停止后台写入。程序退出时调用。
func Close() {
	mu.Lock()
	defer mu.Unlock()
	stopWriteLoopLocked()
	if writer != nil {
		if err := writer.Close(); err != nil {
			slog.Warn("关闭捕获文件失败", "error", err)
		}
		writer = nil
	}
}
