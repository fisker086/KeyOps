package auth

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/fisker086/keyops/pkg/distributed"
	"github.com/fisker086/keyops/pkg/logger"
	pkgredis "github.com/fisker086/keyops/pkg/redis"
)

// RecordingConverter 录制文件转换服务
type RecordingConverter struct {
	converting map[string]bool // 正在转换的文件路径
	mu         sync.RWMutex
}

var (
	converterInstance *RecordingConverter
	converterOnce     sync.Once
)

// GetRecordingConverter 获取转换服务单例
func GetRecordingConverter() *RecordingConverter {
	converterOnce.Do(func() {
		converterInstance = &RecordingConverter{
			converting: make(map[string]bool),
		}
	})
	return converterInstance
}

// ConvertGuacToMP4 将 .guac 文件转换为 MP4
// guacPath: .guac 文件的完整路径
// 返回: MP4 文件路径和错误
func (rc *RecordingConverter) ConvertGuacToMP4(guacPath string) (string, error) {
	// 检查文件是否存在
	if _, err := os.Stat(guacPath); os.IsNotExist(err) {
		return "", fmt.Errorf("guac file not found: %s", guacPath)
	}

	// 生成输出文件路径（将 .guac 替换为 .mp4）
	mp4Path := strings.TrimSuffix(guacPath, ".guac") + ".mp4"

	// 如果 MP4 文件已存在，直接返回
	if _, err := os.Stat(mp4Path); err == nil {
		logger.Infof("[RecordingConverter] MP4 file already exists: %s", mp4Path)
		return mp4Path, nil
	}

	// 使用分布式锁防止多个实例同时转换同一个文件
	lockKey := fmt.Sprintf("zjump:recording:convert:%s", guacPath)
	lock := distributed.NewRedisLock(pkgredis.GetClient(), lockKey, 30*time.Minute) // 锁有效期30分钟

	// 尝试获取锁
	acquired, err := lock.TryLock()
	if err != nil {
		logger.Errorf("[RecordingConverter] Failed to acquire lock for %s: %v", guacPath, err)
		return "", fmt.Errorf("failed to acquire conversion lock: %w", err)
	}

	if !acquired {
		// 如果 Redis 未启用，降级为本地内存锁（单机模式）
		if !pkgredis.IsEnabled() {
			rc.mu.Lock()
			if rc.converting[guacPath] {
				rc.mu.Unlock()
				return "", fmt.Errorf("conversion already in progress for: %s", guacPath)
			}
			rc.converting[guacPath] = true
			rc.mu.Unlock()

			defer func() {
				rc.mu.Lock()
				delete(rc.converting, guacPath)
				rc.mu.Unlock()
			}()
		} else {
			// Redis 已启用但获取锁失败，说明其他实例正在转换
			logger.Infof("[RecordingConverter] Another instance is converting %s, skipping", guacPath)
			return "", fmt.Errorf("conversion already in progress by another instance: %s", guacPath)
		}
	} else {
		// 成功获取分布式锁，转换完成后释放
		defer func() {
			if err := lock.Unlock(); err != nil {
				logger.Errorf("[RecordingConverter] Failed to release lock for %s: %v", guacPath, err)
			}
		}()
	}

	logger.Infof("[RecordingConverter] Starting conversion: %s -> %s", guacPath, mp4Path)

	// 方法1: 使用 guacenc + ffmpeg（推荐）
	// guacenc 输出 .m4v，然后用 ffmpeg 转换为 .mp4
	m4vPath := strings.TrimSuffix(guacPath, ".guac") + ".m4v"

	// 步骤1: 使用 guacenc 转换为 .m4v

	// 检查 guacenc 是否可用
	guacencPath, err := exec.LookPath("guacenc")
	if err != nil {
		logger.Warnf("[RecordingConverter] guacenc not found, RDP recording conversion will not work")
		logger.Warnf("[RecordingConverter] To enable RDP recording conversion, please install guacenc:")
		logger.Warnf("[RecordingConverter]   1. Install guacamole-server package")
		logger.Warnf("[RecordingConverter]   2. Or copy guacenc binary from guacd container to /usr/local/bin/guacenc")
		logger.Warnf("[RecordingConverter]   3. Or use a base image that includes guacenc")
		logger.Warnf("[RecordingConverter] Manual conversion command:")
		logger.Warnf("[RecordingConverter]   Step 1: guacenc -f %s", guacPath)
		logger.Warnf("[RecordingConverter]   Step 2: ffmpeg -i %s.m4v -c:v libx264 -c:a aac -movflags +faststart %s",
			strings.TrimSuffix(guacPath, ".guac"), mp4Path)
		// 方法2: 直接使用 ffmpeg（如果支持，但通常不会成功）
		return rc.convertWithFFmpeg(guacPath, mp4Path)
	}

	// 使用 guacenc 转换
	// guacenc 参数:
	// -s WIDTHxHEIGHT: 分辨率（可选，默认640x480）
	// -r BITRATE: 比特率（可选，默认2000000）
	// -f: 强制转换进行中的录制
	cmd := exec.Command(guacencPath, "-f", guacPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		logger.Errorf("[RecordingConverter] guacenc conversion failed: %v", err)
		// 如果 guacenc 失败，尝试使用 ffmpeg
		return rc.convertWithFFmpeg(guacPath, mp4Path)
	}

	// 检查 .m4v 文件是否生成
	if _, err := os.Stat(m4vPath); os.IsNotExist(err) {
		logger.Warnf("[RecordingConverter] m4v file not generated, trying ffmpeg")
		return rc.convertWithFFmpeg(guacPath, mp4Path)
	}

	// 步骤2: 使用 ffmpeg 将 .m4v 转换为 .mp4
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		logger.Warnf("[RecordingConverter] ffmpeg not found, using m4v file as-is")
		return m4vPath, nil // 返回 .m4v 文件路径
	}

	// 使用 ffmpeg 转换 .m4v 到 .mp4
	// 确保浏览器兼容性：
	// -c:v libx264: 使用 H.264 视频编码（浏览器广泛支持）
	// -c:a aac: 使用 AAC 音频编码（浏览器广泛支持）
	// -preset medium: 编码速度和质量平衡
	// -crf 23: 恒定质量因子（23是高质量）
	// -movflags +faststart: 将 moov atom 移到文件开头（web optimized，支持流式播放）
	// -pix_fmt yuv420p: 像素格式（确保浏览器兼容）
	// -y: 覆盖输出文件
	ffmpegCmd := exec.Command(ffmpegPath,
		"-i", m4vPath,
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-y",
		mp4Path,
	)
	ffmpegCmd.Stdout = os.Stdout
	ffmpegCmd.Stderr = os.Stderr

	if err := ffmpegCmd.Run(); err != nil {
		logger.Errorf("[RecordingConverter] ffmpeg conversion failed: %v", err)
		// 如果转换失败，删除临时 .m4v 文件
		os.Remove(m4vPath)
		return "", fmt.Errorf("ffmpeg conversion failed: %w", err)
	}

	// 删除临时 .m4v 文件
	os.Remove(m4vPath)

	// 检查 MP4 文件是否成功生成
	if _, err := os.Stat(mp4Path); os.IsNotExist(err) {
		return "", fmt.Errorf("mp4 file was not created: %s", mp4Path)
	}

	logger.Infof("[RecordingConverter] Conversion completed successfully: %s", mp4Path)
	return mp4Path, nil
}

// convertWithFFmpeg 使用 ffmpeg 直接转换（备用方案）
// 注意: 这需要 ffmpeg 支持 guac 格式，可能需要先转换为中间格式
func (rc *RecordingConverter) convertWithFFmpeg(guacPath, mp4Path string) (string, error) {
	ffmpegPath, err := exec.LookPath("ffmpeg")
	if err != nil {
		return "", fmt.Errorf("ffmpeg not found: %w", err)
	}

	logger.Infof("[RecordingConverter] Using ffmpeg for conversion: %s -> %s", guacPath, mp4Path)

	// 注意: ffmpeg 可能不直接支持 .guac 格式
	// 这里需要先通过其他方式处理，或者使用 guacenc
	// 如果 guacenc 不可用，可能需要：
	// 1. 使用 guacd 回放并录制为视频
	// 2. 或者使用其他工具

	// 注意: ffmpeg 不直接支持 .guac 格式
	// 这个方案实际上不会工作，应该确保 guacenc 可用
	logger.Warnf("[RecordingConverter] FFmpeg does not support .guac format directly, conversion will fail")
	logger.Warnf("[RecordingConverter] Please install guacenc to enable RDP recording conversion:")
	logger.Warnf("[RecordingConverter]   - Install guacamole-server package, or")
	logger.Warnf("[RecordingConverter]   - Copy guacenc from guacd container: docker cp <guacd-container>:/usr/local/bin/guacenc /usr/local/bin/")
	logger.Warnf("[RecordingConverter] Manual conversion command:")
	logger.Warnf("[RecordingConverter]   Step 1: guacenc -f %s", guacPath)
	logger.Warnf("[RecordingConverter]   Step 2: ffmpeg -i %s.m4v -c:v libx264 -c:a aac -movflags +faststart %s",
		strings.TrimSuffix(guacPath, ".guac"), mp4Path)

	// 尝试使用 ffmpeg（会失败，但提供错误信息）
	// 使用浏览器兼容的编码参数
	cmd := exec.Command(ffmpegPath,
		"-i", guacPath,
		"-c:v", "libx264",
		"-preset", "medium",
		"-crf", "23",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "128k",
		"-movflags", "+faststart",
		"-y",
		mp4Path,
	)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("ffmpeg conversion failed: guac format is not supported. Please install guacenc (from guacamole-server package) to convert RDP recordings: %w", err)
	}

	if _, err := os.Stat(mp4Path); os.IsNotExist(err) {
		return "", fmt.Errorf("mp4 file was not created: %s", mp4Path)
	}

	logger.Infof("[RecordingConverter] FFmpeg conversion completed: %s", mp4Path)
	return mp4Path, nil
}

// ConvertGuacToMP4Async 异步转换 .guac 文件为 MP4
func (rc *RecordingConverter) ConvertGuacToMP4Async(guacPath string) {
	go func() {
		_, err := rc.ConvertGuacToMP4(guacPath)
		if err != nil {
			logger.Errorf("[RecordingConverter] Async conversion failed for %s: %v", guacPath, err)
		}
	}()
}

// IsConverting 检查文件是否正在转换中
// 优先使用 Redis 分布式锁检查，如果 Redis 未启用则使用本地内存检查
func (rc *RecordingConverter) IsConverting(filePath string) bool {
	// 如果 Redis 已启用，使用分布式锁检查
	if pkgredis.IsEnabled() {
		lockKey := fmt.Sprintf("zjump:recording:convert:%s", filePath)
		lock := distributed.NewRedisLock(pkgredis.GetClient(), lockKey, 30*time.Minute)
		isLocked, err := lock.IsLocked()
		if err == nil && isLocked {
			return true
		}
	}

	// Redis 未启用或检查失败，使用本地内存检查（单机模式）
	rc.mu.RLock()
	defer rc.mu.RUnlock()
	return rc.converting[filePath]
}

// StartBackgroundConverter 启动后台转换服务
// 定期扫描录制目录，查找需要转换的 .guac 文件
// 支持分布式部署：使用 Redis 锁确保只有一个实例执行扫描
func (rc *RecordingConverter) StartBackgroundConverter(recordingBasePath string, interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	logger.Infof("[RecordingConverter] Background converter started, scanning: %s, interval: %v", recordingBasePath, interval)

	for range ticker.C {
		// 如果启用了 Redis，使用分布式锁确保只有一个实例执行扫描
		if pkgredis.IsEnabled() {
			rc.scanAndConvertWithLock(recordingBasePath)
		} else {
			// Redis 未启用，直接执行扫描（单机模式）
			rc.scanAndConvert(recordingBasePath)
		}
	}
}

// scanAndConvertWithLock 使用分布式锁的扫描转换
func (rc *RecordingConverter) scanAndConvertWithLock(basePath string) {
	// 创建分布式锁，锁的有效期为扫描间隔的2倍
	lockKey := "zjump:recording:converter:scan"
	lock := distributed.NewRedisLock(pkgredis.GetClient(), lockKey, 10*time.Minute)

	// 尝试获取锁
	acquired, err := lock.TryLock()
	if err != nil {
		logger.Errorf("[RecordingConverter] Failed to acquire scan lock: %v", err)
		return
	}

	if !acquired {
		logger.Infof("[RecordingConverter] Another instance is scanning, skipping...")
		return
	}

	defer func() {
		if err := lock.Unlock(); err != nil {
			logger.Errorf("[RecordingConverter] Failed to release scan lock: %v", err)
		}
	}()

	logger.Infof("[RecordingConverter] Acquired distributed scan lock, starting scan...")
	rc.scanAndConvert(basePath)
}

// scanAndConvert 扫描目录并转换 .guac 文件
func (rc *RecordingConverter) scanAndConvert(basePath string) {
	// 遍历录制目录
	err := filepath.Walk(basePath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // 忽略错误，继续扫描
		}

		// 只处理 .guac 文件
		if !strings.HasSuffix(strings.ToLower(path), ".guac") {
			return nil
		}

		// 检查是否已经有对应的 MP4 文件
		mp4Path := strings.TrimSuffix(path, ".guac") + ".mp4"
		if _, err := os.Stat(mp4Path); err == nil {
			return nil // MP4 已存在，跳过
		}

		// 检查是否正在转换中
		if rc.IsConverting(path) {
			return nil // 正在转换，跳过
		}

		// 检查文件是否还在写入中（通过文件修改时间判断）
		// 如果文件在最近1分钟内被修改，可能还在录制中，跳过
		if time.Since(info.ModTime()) < time.Minute {
			return nil
		}

		// 触发异步转换
		logger.Infof("[RecordingConverter] Found unconverted guac file: %s", path)
		rc.ConvertGuacToMP4Async(path)

		return nil
	})

	if err != nil {
		logger.Errorf("[RecordingConverter] Error scanning directory: %v", err)
	}
}
