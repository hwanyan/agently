//go:build ignore
// +build ignore

package main

import (
	"context"
	"fmt"
	"log"
	"os"

	"agent-hwan/internal/llm"
	"agent-hwan/pkg/types"
)

func main() {
	// 从环境变量获取API密钥
	apiKey := os.Getenv("DEEPSEEK_API_KEY")
	if apiKey == "" {
		log.Fatal("请设置DEEPSEEK_API_KEY环境变量")
	}

	// 创建客户端
	client := llm.NewDeepSeekClient(apiKey, "deepseek-chat", "")

	// 验证配置
	if err := client.ValidateConfig(); err != nil {
		log.Fatalf("配置验证失败: %v", err)
	}

	// 测试连接
	ctx := context.Background()
	fmt.Println("测试DeepSeek连接...")
	if err := client.TestConnection(ctx); err != nil {
		log.Fatalf("连接测试失败: %v", err)
	}
	fmt.Println("连接测试成功！")

	// 示例1: 基本对话
	fmt.Println("\n=== 基本对话示例 ===")
	deepseekBasicChat(ctx, client)

	// 示例2: 多轮对话
	fmt.Println("\n=== 多轮对话示例 ===")
	deepseekMultiTurnChat(ctx, client)

	// 示例3: 流式对话
	fmt.Println("\n=== 流式对话示例 ===")
	deepseekStreamChat(ctx, client)

	// 示例4: 代码生成
	fmt.Println("\n=== 代码生成示例 ===")
	deepseekCodeGeneration(ctx, client)
}

// 基本对话示例
func deepseekBasicChat(ctx context.Context, client *llm.DeepSeekClient) {
	messages := []types.Message{
		{
			Role:    "user",
			Content: "你好，请介绍一下DeepSeek AI。",
		},
	}

	response, err := client.Chat(ctx, messages, nil)
	if err != nil {
		log.Printf("基本对话失败: %v", err)
		return
	}

	fmt.Printf("用户: %s\n", messages[0].Content)
	fmt.Printf("DeepSeek: %s\n", response.Content)
	fmt.Printf("预估Token数: %d\n", client.EstimateTokens(response.Content))
}

// 多轮对话示例
func deepseekMultiTurnChat(ctx context.Context, client *llm.DeepSeekClient) {
	messages := []types.Message{
		{
			Role:    "user",
			Content: "我想学习Go语言，你能给我一些建议吗？",
		},
	}

	// 第一轮对话
	response1, err := client.Chat(ctx, messages, nil)
	if err != nil {
		log.Printf("第一轮对话失败: %v", err)
		return
	}

	fmt.Printf("用户: %s\n", messages[0].Content)
	fmt.Printf("DeepSeek: %s\n", response1.Content)

	// 添加助手的回复到对话历史
	messages = append(messages, *response1)

	// 第二轮对话
	messages = append(messages, types.Message{
		Role:    "user",
		Content: "那么从哪里开始学习比较好呢？",
	})

	response2, err := client.Chat(ctx, messages, nil)
	if err != nil {
		log.Printf("第二轮对话失败: %v", err)
		return
	}

	fmt.Printf("用户: %s\n", messages[2].Content)
	fmt.Printf("DeepSeek: %s\n", response2.Content)
}

// 流式对话示例
func deepseekStreamChat(ctx context.Context, client *llm.DeepSeekClient) {
	messages := []types.Message{
		{
			Role:    "user",
			Content: "请写一首关于人工智能的诗，要求有韵律感。",
		},
	}

	fmt.Printf("用户: %s\n", messages[0].Content)
	fmt.Print("DeepSeek: ")

	stream, err := client.ChatStream(ctx, messages, nil)
	if err != nil {
		log.Printf("流式对话失败: %v", err)
		return
	}

	var fullContent string
	for chunk := range stream {
		fmt.Print(chunk)
		fullContent += chunk
	}
	fmt.Println()
}

// 代码生成示例
func deepseekCodeGeneration(ctx context.Context, client *llm.DeepSeekClient) {
	// 切换到代码模型（如果可用）
	originalModel := client.GetModel()
	if deepseekContains(client.GetSupportedModels(), "deepseek-coder") {
		client.SetModel("deepseek-coder")
		defer client.SetModel(originalModel) // 恢复原模型
	}

	messages := []types.Message{
		{
			Role: "user",
			Content: `请用Go语言实现一个简单的HTTP服务器，要求：
1. 监听8080端口
2. 提供一个/hello接口，返回JSON格式的问候消息
3. 提供一个/health接口，返回服务状态
4. 包含适当的错误处理和日志记录`,
		},
	}

	response, err := client.Chat(ctx, messages, nil)
	if err != nil {
		log.Printf("代码生成失败: %v", err)
		return
	}

	fmt.Printf("用户需求: %s\n", messages[0].Content)
	fmt.Println("生成的代码:")
	fmt.Println(response.Content)
	fmt.Printf("预估Token数: %d\n", client.EstimateTokens(response.Content))
	fmt.Printf("预估成本: $%.6f\n", client.CalculateCost(
		client.EstimateTokens(messages[0].Content),
		client.EstimateTokens(response.Content),
	))
}

// 辅助函数：检查切片是否包含指定元素
func deepseekContains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}
