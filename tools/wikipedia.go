package tools

import (
	"context"
	"time"

	"github.com/cloudwego/eino-ext/components/tool/wikipedia"
	"github.com/cloudwego/eino/components/tool"
	"github.com/tk103331/eino-cli/config"
)

// NewWikipediaTool 创建Wikipedia工具
func NewWikipediaTool(name string, cfg config.Tool) (tool.InvokableTool, error) {
	ctx := context.Background()

	// 创建默认配置
	wikiConfig := &wikipedia.Config{
		Language:    "zh",             // 默认中文
		TopK:        5,                // 默认返回5个结果
		DocMaxChars: 500,              // 默认摘要最大长度
		Timeout:     15 * time.Second, // 默认超时时间
		MaxRedirect: 3,                // 默认最大重定向次数
	}

	// 从配置中读取参数
	if language, exists := cfg.Config["language"]; exists {
		wikiConfig.Language = language.String()
	}
	if topK, exists := cfg.Config["top_k"]; exists {
		if val := topK.Int(); val > 0 {
			wikiConfig.TopK = val
		}
	}
	if docMaxChars, exists := cfg.Config["doc_max_chars"]; exists {
		if val := docMaxChars.Int(); val > 0 {
			wikiConfig.DocMaxChars = val
		}
	}
	if timeoutConfig, exists := cfg.Config["timeout"]; exists {
		if val := timeoutConfig.Int(); val > 0 {
			wikiConfig.Timeout = time.Duration(val) * time.Second
		}
	}
	if baseURL, exists := cfg.Config["base_url"]; exists {
		wikiConfig.BaseURL = baseURL.String()
	}
	if userAgent, exists := cfg.Config["user_agent"]; exists {
		wikiConfig.UserAgent = userAgent.String()
	}
	if maxRedirect, exists := cfg.Config["max_redirect"]; exists {
		if val := maxRedirect.Int(); val > 0 {
			wikiConfig.MaxRedirect = val
		}
	}

	// 创建工具实例
	return wikipedia.NewTool(ctx, wikiConfig)
}
