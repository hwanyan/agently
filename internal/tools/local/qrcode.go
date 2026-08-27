// 通用智能助手 agent 的工具：二维码生成工具（本地工具，随项目启动）
package local

import (
	"context"
	"encoding/base64"
	"errors"
	"fmt"
	"github.com/hwanyan/agently/pkg/types"
	"os"
	"strings"

	"github.com/skip2/go-qrcode"
)

// 工具自注册
//
// 注意：QRCodeTool 不依赖任何配置，此处仅需同步 ToolFactory 的签名。
func init() {
	Register("general", func(_ types.ConfigProvider) types.Tool {
		return NewQRCodeTool()
	})
}

// QRCodeTool 常量定义
const (
	// qrcode 工具名称
	QRCodeToolName = "qrcode"

	// qrcode 工具描述
	QRCodeToolDesc = "二维码生成工具，支持将文本生成二维码，输出为DataURL、ASCII或保存到文件。"
)

// QRCodeTool 二维码生成工具
type QRCodeTool struct{}

// NewQRCodeTool 创建二维码工具实例
func NewQRCodeTool() *QRCodeTool {
	return &QRCodeTool{}
}

// Name 返回工具名称
func (q *QRCodeTool) Name() string { return QRCodeToolName }

// Description 返回工具描述
func (q *QRCodeTool) Description() string { return QRCodeToolDesc }

// Parameters 返回参数结构（用于 FunctionCall 的参数约束）
func (q *QRCodeTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"text": map[string]interface{}{
				"type":        "string",
				"description": "需要编码为二维码的文本内容，例如URL或任意文本",
			},
			"size": map[string]interface{}{
				"type":        "integer",
				"description": "二维码图片尺寸（像素），默认256✖️256",
				"default":     256,
				"minimum":     64,
				"maximum":     1024,
			},
			"level": map[string]interface{}{
				"type":        "string",
				"description": "容错级别：L(7%)、M(15%)、Q(25%)、H(30%)。默认M",
				"default":     "M",
				"enum":        []string{"L", "M", "Q", "H"},
			},
			"margin": map[string]interface{}{
				"type":        "integer",
				"description": "二维码边距（像素），默认4",
				"default":     4,
				"minimum":     0,
				"maximum":     64,
			},
			"output": map[string]interface{}{
				"type":        "string",
				"description": "输出格式：dataurl，返回Base64的data URL（选择此格式时需要在终端直接展示）、ascii，终端ASCII预览（选择此格式时需要在终端直接展示）、file（选择此格式需要将文件保存至给定位置）",
				"default":     "dataurl",
				"enum":        []string{"dataurl", "ascii", "file"},
			},
			"filename": map[string]interface{}{
				"type":        "string",
				"description": "当 output=file 时保存的文件名（例如 qr.png）。不传则自动生成。",
			},
		},
		"required": []string{"text"},
	}
}

// Execute 执行二维码生成
func (q *QRCodeTool) Execute(ctx context.Context, args map[string]interface{}) (string, error) {
	// 解析参数
	var p struct {
		Text     string `json:"text"`
		Size     int    `json:"size"`
		Level    string `json:"level"`
		Margin   int    `json:"margin"`
		Output   string `json:"output"`
		Filename string `json:"filename"`
	}
	// 设置默认值
	p.Size = 256
	p.Level = "M"
	p.Margin = 4
	p.Output = "dataurl"

	// 从 map 中解析参数
	if v, ok := args["text"].(string); ok {
		p.Text = v
	}
	if v, ok := args["size"].(float64); ok {
		p.Size = int(v)
	}
	if v, ok := args["level"].(string); ok {
		p.Level = v
	}
	if v, ok := args["margin"].(float64); ok {
		p.Margin = int(v)
	}
	if v, ok := args["output"].(string); ok {
		p.Output = v
	}
	if v, ok := args["filename"].(string); ok {
		p.Filename = v
	}
	if strings.TrimSpace(p.Text) == "" {
		return "", errors.New("参数错误: text 不能为空")
	}
	if p.Size < 64 || p.Size > 1024 {
		return "", fmt.Errorf("参数错误: size 超出范围 (64~1024)")
	}
	if p.Margin < 0 || p.Margin > 64 {
		return "", fmt.Errorf("参数错误: margin 超出范围 (0~64)")
	}
	var level qrcode.RecoveryLevel
	switch strings.ToUpper(p.Level) {
	case "L":
		level = qrcode.Low
	case "M":
		level = qrcode.Medium
	case "Q":
		level = qrcode.High
	case "H":
		level = qrcode.Highest
	default:
		return "", fmt.Errorf("参数错误: 不支持的 level 值 '%s'", p.Level)
	}

	// 生成二维码
	qr, err := qrcode.New(p.Text, level)
	if err != nil {
		return "", fmt.Errorf("生成二维码失败: %w", err)
	}
	qr.DisableBorder = (p.Margin == 0)

	// 根据 output 处理结果
	switch p.Output {
	case "dataurl":
		png, err := qr.PNG(p.Size)
		if err != nil {
			return "", fmt.Errorf("生成PNG失败: %w", err)
		}
		dataURL := "data:image/png;base64," + base64.StdEncoding.EncodeToString(png)
		return fmt.Sprintf("二维码(Data URL):\n%s", dataURL), nil

	case "ascii":
		// 简易ASCII渲染（非精准像素），用于终端预览
		matrix := qr.Bitmap()
		var b strings.Builder
		b.WriteString("二维码(ASCII)预览:\n")
		for y := range matrix {
			for x := range matrix[y] {
				if matrix[y][x] {
					b.WriteString("██")
				} else {
					b.WriteString("  ")
				}
			}
			b.WriteString("\n")
		}
		return b.String(), nil

	case "file":
		filename := strings.TrimSpace(p.Filename)
		if filename == "" {
			filename = "qr.png"
		}
		// 防止路径穿越，限制为当前工作目录
		if strings.Contains(filename, "..") {
			return "", fmt.Errorf("文件名非法")
		}
		if err := qrcode.WriteFile(p.Text, level, p.Size, filename); err != nil {
			return "", fmt.Errorf("保存文件失败: %w", err)
		}
		abs, _ := os.Getwd()
		return fmt.Sprintf("二维码已保存: %s/%s", abs, filename), nil

	default:
		return "", fmt.Errorf("参数错误: 不支持的 output 值 '%s'", p.Output)
	}
}
