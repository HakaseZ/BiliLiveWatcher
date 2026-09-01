// Command replay 回放捕获的原始数据包文件,逐帧调用 ParsePacket 输出解析结果。
//
// 用途:服务器更新数据格式后,用旧捕获数据配合新解析器反复运行,辅助逆向分析;
// 也可用于解析器改动后的真实数据回归验证。
//
// 用法:
//
//	go run ./cmd/replay logs/capture.bin
//	go run ./cmd/replay -raw logs/capture.bin
package main

import (
	"bufio"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	m "github.com/HakaseZ/BiliLiveWatcher/watchers/messageReader"
)

const (
	magic           = 0x424c4350 // "BLCP"
	frameHeaderSize = 4 + 8 + 4  // magic + 时间戳 + 包长度
	maxPacketSize   = 16 << 20   // 原始包长度上限,防止损坏数据导致内存膨胀
)

func main() {
	raw := flag.Bool("raw", false, "同时打印每个数据包的原始字节(hex)")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "用法: replay [-raw] <捕获文件...>\n回放捕获的原始数据包并打印解析结果。\n\n选项:\n")
		flag.PrintDefaults()
	}
	flag.Parse()
	if flag.NArg() == 0 {
		flag.Usage()
		os.Exit(2)
	}
	exitCode := 0
	for _, path := range flag.Args() {
		if err := replayFile(path, *raw); err != nil {
			fmt.Fprintf(os.Stderr, "replay %s: %v\n", path, err)
			exitCode = 1
		}
	}
	os.Exit(exitCode)
}

func replayFile(path string, raw bool) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	r := bufio.NewReader(f)
	var (
		header   [frameHeaderSize]byte
		frameNum int
		skipped  int64
	)
	for {
		if _, err := io.ReadFull(r, header[:]); err != nil {
			if err == io.EOF {
				break
			}
			if err == io.ErrUnexpectedEOF {
				fmt.Fprintf(os.Stderr, "警告: 第 %d 帧后文件被截断,忽略残留数据\n", frameNum)
				break
			}
			return err
		}
		if binary.BigEndian.Uint32(header[0:4]) != magic {
			n, err := resync(r)
			skipped += n
			if err != nil {
				fmt.Fprintf(os.Stderr, "警告: 无法重新同步到下一个帧头(共跳过 %d 字节),终止\n", skipped)
				break
			}
			fmt.Fprintf(os.Stderr, "警告: 检测到损坏帧,跳过 %d 字节\n", n)
			continue
		}
		ts := time.Unix(0, int64(binary.BigEndian.Uint64(header[4:12])))
		pktLen := binary.BigEndian.Uint32(header[12:16])
		if pktLen < 16 || pktLen > maxPacketSize {
			fmt.Fprintf(os.Stderr, "警告: 第 %d 帧长度非法(%d),重新同步\n", frameNum, pktLen)
			if _, err := resync(r); err != nil {
				break
			}
			continue
		}
		packet := make([]byte, pktLen)
		if _, err := io.ReadFull(r, packet); err != nil {
			fmt.Fprintf(os.Stderr, "警告: 第 %d 帧数据读取失败: %v,终止\n", frameNum, err)
			break
		}
		frameNum++
		printFrame(ts, packet, raw)
	}
	fmt.Printf("共 %d 帧\n", frameNum)
	return nil
}

// resync 向后逐字节扫描,定位下一个 magic 帧头,返回跳过的垃圾字节数。
// 匹配时流位置停留在 magic 首字节处,由后续 ReadFull 从 "BLCP" 重新读取。
func resync(r *bufio.Reader) (int64, error) {
	const pattern = "BLCP"
	skipped := int64(0)
	for {
		b, err := r.Peek(4)
		if err != nil {
			r.Discard(len(b))
			return skipped + int64(len(b)), err
		}
		if string(b) == pattern {
			return skipped, nil
		}
		r.Discard(1)
		skipped++
	}
}

func printFrame(ts time.Time, packet []byte, raw bool) {
	pktLen := binary.BigEndian.Uint32(packet[0:4])
	ver := binary.BigEndian.Uint16(packet[6:8])
	op := binary.BigEndian.Uint32(packet[8:12])

	results := m.ParsePacket(packet)
	out := fmt.Sprintf("[%s] len=%d ver=%d op=%d", ts.Format("2006-01-02 15:04:05.000000000"), pktLen, ver, op)
	if len(results) == 0 {
		out += " → (无业务消息)"
	} else {
		var parts []string
		for _, res := range results {
			if res.Mtype == "" {
				parts = append(parts, "(无业务消息)")
				continue
			}
			msg, err := json.Marshal(res.Msg)
			if err != nil {
				msg = []byte(fmt.Sprintf("%+v", res.Msg))
			}
			parts = append(parts, fmt.Sprintf("%s %s", res.Mtype, msg))
		}
		out += " → " + strings.Join(parts, " | ")
	}
	fmt.Println(out)
	if raw {
		fmt.Printf("    %s\n", hex.EncodeToString(packet))
	}
}
