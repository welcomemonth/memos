package chatReAct

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/cloudwego/eino/components/tool"
	"github.com/cloudwego/eino/components/tool/utils"
	"github.com/cloudwego/eino/schema"
)

// 接口方式实现一个搜索工具
type SerpapiImpl struct {
	config *SerpapiConfig
	client *http.Client
}

type SerpapiConfig struct {
	APIKey   string // 必填：SERPAPI_KEY
	BaseUrl  string // 可选，默认 https://serpapi.com/search
	Location string // 默认搜索地点
	Number   int    // 默认返回数量
	Language string // 语言，如 zh-cn / en
}

type SerpapiInput struct {
	Query string `json:"query"`
	Num   int    `json:"num,omitempty"`
}

func newSerpapiTool(ctx context.Context) (bt tool.BaseTool, err error) {
	apiKey := os.Getenv("SERPAPI_KEY")
	if apiKey == "" {
		return nil, fmt.Errorf("环境变量 SERPAPI_KEY 未设置")
	}

	config := &SerpapiConfig{
		APIKey:   apiKey,
		BaseUrl:  "https://serpapi.com/search",
		Location: "Shanghai, China",
		Number:   10,
		Language: "zh-cn",
	}

	return &SerpapiImpl{
		config: config,
		client: &http.Client{Timeout: 10 * time.Second},
	}, nil
}

// Info 返回工具元信息（模型靠这个决定是否调用）
func (impl *SerpapiImpl) Info(ctx context.Context) (*schema.ToolInfo, error) {
	return &schema.ToolInfo{
		Name: "web_search",
		Desc: "搜索互联网获取最新信息、新闻、事实。当需要实时数据、最新事件、无法从已有知识回答的问题时，请使用此工具。",
		ParamsOneOf: schema.NewParamsOneOfByParams(map[string]*schema.ParameterInfo{
			"query": {
				Type:     schema.String,
				Desc:     "要搜索的关键词或完整问题，例如：'2026年奥运会举办城市' 或 'golang eino 框架使用教程'",
				Required: true,
			},
			"num": {
				Type: schema.Integer,
				Desc: "希望返回的结果数量，建议 1~10，默认 5",
			},
		}),
	}, nil
}

func (impl *SerpapiImpl) InvokableRun(ctx context.Context, argumentsInJSON string, opts ...tool.Option) (string, error) {
	var input SerpapiInput
	if err := json.Unmarshal([]byte(argumentsInJSON), &input); err != nil {
		return "", fmt.Errorf("解析参数失败: %w, 原始参数: %s", err, argumentsInJSON)
	}
	if strings.TrimSpace(input.Query) == "" {
		return "", fmt.Errorf("query 不能为空")
	}
	// 2. 确定返回数量
	num := impl.config.Number
	if input.Num > 0 {
		num = input.Num
	}
	if num > 10 {
		num = 5 // 限制上限，避免浪费额度
	}
	// 3. 构造请求 URL
	params := url.Values{}
	params.Set("api_key", impl.config.APIKey)
	params.Set("engine", "google")
	params.Set("q", input.Query)
	params.Set("num", strconv.Itoa(num))
	params.Set("hl", impl.config.Language) // 界面语言
	params.Set("gl", "cn")                 // 国家/地区，可按需调整

	if impl.config.Location != "" {
		params.Set("location", impl.config.Location)
	}

	reqURL := impl.config.BaseUrl + "?" + params.Encode()

	// 4. 发送 HTTP 请求
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, reqURL, nil)
	if err != nil {
		return "", fmt.Errorf("创建请求失败: %w", err)
	}

	resp, err := impl.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("请求 SerpAPI 失败: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("读取响应失败: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("SerpAPI 返回错误状态码 %d: %s", resp.StatusCode, string(body))
	}

	// 5. 解析 SerpAPI 返回结果，提取有用信息
	var serpResp map[string]any
	if err := json.Unmarshal(body, &serpResp); err != nil {
		return "", fmt.Errorf("解析 SerpAPI 响应失败: %w", err)
	}

	// 提取 organic_results
	type simplifiedResult struct {
		Position int    `json:"position"`
		Title    string `json:"title"`
		Link     string `json:"link"`
		Snippet  string `json:"snippet"`
	}

	var results []simplifiedResult
	if organic, ok := serpResp["organic_results"].([]any); ok {
		for _, item := range organic {
			if m, ok := item.(map[string]any); ok {
				r := simplifiedResult{}
				if v, ok := m["position"].(float64); ok {
					r.Position = int(v)
				}
				if v, ok := m["title"].(string); ok {
					r.Title = v
				}
				if v, ok := m["link"].(string); ok {
					r.Link = v
				}
				if v, ok := m["snippet"].(string); ok {
					r.Snippet = v
				}
				results = append(results, r)
			}
		}
	}

	// 6. 构造返回给模型的最终结果（必须是字符串）
	output := map[string]any{
		"query":   input.Query,
		"results": results,
	}

	// 如果有 answer_box 也可以加上
	if ab, ok := serpResp["answer_box"]; ok {
		output["answer_box"] = ab
	}

	resultBytes, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return "", fmt.Errorf("序列化结果失败: %w", err)
	}
	slog.Debug("SearchApi Result: ", string(resultBytes))
	return string(resultBytes), nil
}

// ==============================================================================================
// 使用 InferTool 创建计算器工具
type CalculateInput struct {
	Operation string  `json:"operation" jsonschema:"required,enum=add,enum=subtract,enum=multiply,enum=divide" jsonschema_description:"运算类型：add(加)、subtract(减)、multiply(乘)、divide(除)"`
	Number1   float64 `json:"number1" jsonschema:"required" jsonschema_description:"第一个数字"`
	Number2   float64 `json:"number2" jsonschema:"required" jsonschema_description:"第二个数字"`
}

// ==================== 2. 输出结构体 ====================
type CalculateOutput struct {
	Result     float64 `json:"result"`
	Expression string  `json:"expression"` // 方便模型理解，例如 "10 + 5 = 15"
	Operation  string  `json:"operation"`
}

// ==================== 3. 具体计算逻辑 ====================
func Calculate(ctx context.Context, input *CalculateInput) (*CalculateOutput, error) {
	var result float64
	var opSymbol string

	switch input.Operation {
	case "add":
		result = input.Number1 + input.Number2
		opSymbol = "+"
	case "subtract":
		result = input.Number1 - input.Number2
		opSymbol = "-"
	case "multiply":
		result = input.Number1 * input.Number2
		opSymbol = "*"
	case "divide":
		if input.Number2 == 0 {
			return nil, fmt.Errorf("除数不能为 0")
		}
		result = input.Number1 / input.Number2
		opSymbol = "/"
	default:
		return nil, fmt.Errorf("不支持的运算类型: %s，仅支持 add/subtract/multiply/divide", input.Operation)
	}

	expression := fmt.Sprintf("%v %s %v = %v", input.Number1, opSymbol, input.Number2, result)
	slog.Debug("Calculate Result: ", expression)

	return &CalculateOutput{
		Result:     result,
		Expression: expression,
		Operation:  input.Operation,
	}, nil
}

// ============ 4. 使用InferTool 创建工具
func CreateCalculateTool() (tool.InvokableTool, error) {
	return utils.InferTool(
		"calculator", // 工具名称
		"执行基本的数学计算（加减乘除）。当需要精确计算两个数字的加减乘除时使用此工具。", // 工具描述（非常重要）
		Calculate, // 具体的业务函数
	)
}
