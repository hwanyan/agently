package config

import (
	"fmt"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/hwanyan/agently/internal/logger"
	"github.com/hwanyan/agently/pkg/types"
)

// Manager 负责配置的加载、热更新（watch 模式）与线程安全访问
//
// 设计说明：
//   - 内部使用 atomic.Pointer 保存当前生效的配置快照，读多写少场景下无锁读取，开销极小
//   - 通过 fsnotify 监听配置文件所在【目录】而非文件本身：许多编辑器/工具保存文件时会执行
//     "写临时文件 + rename 覆盖" 的原子替换操作，直接监听文件本身在 rename 后会失效（部分平台上
//     inode 已变化，后续事件不会再推送到旧的 watch 描述符），监听目录可以规避这个问题
//   - 重新加载时会完整解析并校验（复用启动期的 validateConfig），校验失败则保留旧配置并打印
//     错误日志，不会让运行中的服务因为一次错误的配置编辑而崩溃或进入不一致状态（fail-safe，
//     区别于启动期"校验失败即 fail-fast"的策略）
//   - 短时间内的多次写入事件（编辑器保存过程中可能触发多次 fsnotify 事件）会被 debounce
//     合并为一次重载，避免重复解析
//
// 使用方式：
//
//	cm, err := config.NewManager("./config.yaml")
//	...
//	cm.Watch()           // 启动后台监听，非阻塞
//	cfg := cm.Get()      // 任意时刻获取当前最新配置快照
type Manager struct {
	path       string // 配置文件绝对路径
	dir        string // 配置文件所在目录（watch 目标）
	cfg        atomic.Pointer[types.Config]
	watcher    *fsnotify.Watcher
	stopCh     chan struct{}
	debounceMs time.Duration // 防抖延迟窗口（指定文件变化后需要“等待”多久再执行重载）
}

// NewManager 创建配置管理器并完成首次加载
// 首次加载失败即返回 error（fail-fast），复用 loadConfigFromFile 完成配置解析与校验
func NewManager(path string) (*Manager, error) {
	if path == "" {
		path = "./config.yaml"
	}
	absPath, err := filepath.Abs(path)
	if err != nil {
		return nil, fmt.Errorf("解析配置文件路径失败: %w", err)
	}

	cfg, err := loadConfig(absPath)
	if err != nil {
		return nil, fmt.Errorf("加载配置文件失败: %w", err)
	}
	cfg.Log.FilePath = deriveLogFilePath(absPath)

	m := &Manager{
		path:       absPath,
		dir:        filepath.Dir(absPath),
		stopCh:     make(chan struct{}),
		debounceMs: 300 * time.Millisecond,
	}
	m.cfg.Store(cfg)
	return m, nil
}

// Get 获取当前生效的配置快照（线程安全，无锁读取）
//
// 注意：返回值是某一时刻的完整只读快照。调用方应在【每次】需要用到配置值时重新调用
// Get()，而不要长期持有旧快照跨越多次调用复用，否则无法感知到之后发生的热更新。
func (m *Manager) Get() *types.Config {
	return m.cfg.Load()
}

// Path 返回配置文件的绝对路径
func (m *Manager) Path() string {
	return m.path
}

// Watch 启动文件监听，检测到配置文件变化后自动重新加载并热更新到内存
// 该方法内部启动一个常驻 goroutine，非阻塞返回。
func (m *Manager) Watch() error {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return fmt.Errorf("创建配置文件监听器失败: %w", err)
	}
	if err = watcher.Add(m.dir); err != nil {
		_ = watcher.Close()
		return fmt.Errorf("监听配置文件目录失败: %w", err)
	}
	m.watcher = watcher

	go m.watchLoop()
	logger.Info("配置热更新监听已启动: %s", m.path)
	return nil
}

// watchLoop 监听循环：debounce 合并事件后触发重载
func (m *Manager) watchLoop() {
	var debounceTimer *time.Timer

	for {
		select {
		case event, ok := <-m.watcher.Events:
			if !ok {
				return
			}
			// 只关心目标配置文件自身的变化；目录下其他文件的事件忽略
			if filepath.Clean(event.Name) != m.path {
				continue
			}
			// 关心写入 / 创建 / 重命名（覆盖式原子写会产生 rename 事件）
			if event.Op&(fsnotify.Write|fsnotify.Create|fsnotify.Rename) == 0 {
				continue
			}
			if debounceTimer != nil {
				debounceTimer.Stop()
			}
			// 启动一个计时器，在等待 m.debounceMs 时间后，在新的 goroutine 中异步调用 m.reloadSafely，
			// 执行配置重载逻辑。如果在这个等待时间内再次收到新的事件，之前的定时器会被重置
			debounceTimer = time.AfterFunc(m.debounceMs, m.reloadSafely)
		case err, ok := <-m.watcher.Errors:
			if !ok {
				return
			}
			logger.Error("配置文件监听出现错误: %v", err)
		case <-m.stopCh:
			return
		}
	}
}

// reloadSafely 重新加载配置并处理错误（供 debounce 定时器回调使用）
func (m *Manager) reloadSafely() {
	if err := m.reload(); err != nil {
		logger.Error("配置热更新失败，继续使用旧配置: %v", err)
	}
}

// reload 重新加载配置文件并校验，成功后原子替换内存中的配置
func (m *Manager) reload() error {
	newCfg, err := loadConfig(m.path)
	if err != nil {
		return err
	}
	newCfg.Log.FilePath = deriveLogFilePath(m.path)
	m.cfg.Store(newCfg)
	logger.Info("配置热更新成功，已生效: %s", m.path)
	return nil
}

// deriveLogFilePath 根据配置文件绝对路径反推日志文件路径前缀
//
// 日志路径不再从配置文件中读取，而是基于约定的部署目录结构自动生成：约定配置文件所在目录
// 与日志目录 logs 互为同级（同处于某个"项目根目录"下），即：配置文件路径为
// <项目根>/<任意子目录>/config.yaml 时，日志路径为 <项目根>/logs/agently。
//
// 该约定同时兼容两种常见部署布局：
//   - 可执行文件与 config.yaml 同处于 bin 目录（<项目根>/bin/{agently,config.yaml}）；
//   - 可执行文件与 config.yaml 分离部署（<项目根>/bin/agently、<项目根>/config/config.yaml），
//     此时通过 -config 传入配置文件路径或所在目录（见 main.go resolveConfigPath），
//     只要 config 目录与 bin 目录同为项目根的直接子目录，推导结果一致，无需额外配置日志路径。
func deriveLogFilePath(configAbsPath string) string {
	configDir := filepath.Dir(configAbsPath)
	projectRoot := filepath.Dir(configDir)
	return filepath.Join(projectRoot, "logs", "agently")
}

// Close 停止文件监听，释放相关资源
func (m *Manager) Close() error {
	select {
	case <-m.stopCh:
		// 已关闭，避免重复 close 导致 panic
	default:
		close(m.stopCh)
	}
	if m.watcher != nil {
		return m.watcher.Close()
	}
	return nil
}
