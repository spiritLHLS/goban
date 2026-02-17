package bili

import (
	"fmt"
	"log"
	"math"
	"strings"
	"time"

	"github.com/imroc/req/v3"
)

type BiliClient struct {
	Cookies       string
	UID           int64
	ReqClient     *req.Client
	MaxRetries    int // 最大重试次数
	RetryInterval int // 重试基础间隔（秒）
}

func NewBiliClient(cookies string, uid int64) *BiliClient {
	client := req.C().
		SetTimeout(30 * time.Second).
		EnableKeepAlives().
		ImpersonateChrome()

	if cookies != "" {
		client.SetCommonHeader("Cookie", cookies)
	}

	return &BiliClient{
		Cookies:       cookies,
		UID:           uid,
		ReqClient:     client,
		MaxRetries:    3,
		RetryInterval: 2,
	}
}

// NewBiliClientWithProxy 创建带代理的BiliClient
func NewBiliClientWithProxy(cookies string, uid int64, proxyURL string) *BiliClient {
	client := req.C().
		SetTimeout(30 * time.Second).
		EnableKeepAlives().
		ImpersonateChrome()

	if cookies != "" {
		client.SetCommonHeader("Cookie", cookies)
	}

	// 设置代理
	if proxyURL != "" {
		client.SetProxyURL(proxyURL)
	}

	return &BiliClient{
		Cookies:       cookies,
		UID:           uid,
		ReqClient:     client,
		MaxRetries:    3,
		RetryInterval: 2,
	}
}

// SetRetryPolicy 设置重试策略
func (c *BiliClient) SetRetryPolicy(maxRetries, retryInterval int) {
	c.MaxRetries = maxRetries
	c.RetryInterval = retryInterval
}

// retryWithBackoff 使用指数退避策略重试函数
func (c *BiliClient) retryWithBackoff(operation func() error) error {
	var lastErr error
	
	for attempt := 0; attempt <= c.MaxRetries; attempt++ {
		lastErr = operation()
		
		if lastErr == nil {
			return nil
		}
		
		if attempt < c.MaxRetries {
			// 计算退避时间：基础间隔 * 2^尝试次数
			backoffTime := time.Duration(c.RetryInterval) * time.Second * time.Duration(math.Pow(2, float64(attempt)))
			log.Printf("[重试] 第 %d 次尝试失败，%v 后重试: %v", attempt+1, backoffTime, lastErr)
			time.Sleep(backoffTime)
		}
	}
	
	return fmt.Errorf("重试 %d 次后仍然失败: %w", c.MaxRetries, lastErr)
}

// UserInfoResponse 用户信息响应
type UserInfoResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Mid   int64  `json:"mid"`
		Uname string `json:"uname"`
		Face  string `json:"face"`
		Level int    `json:"level_info.current_level"`
	} `json:"data"`
}

// GetUserInfo 获取用户信息
func GetUserInfo(cookies string) (*UserInfoResponse, error) {
	apiURL := "https://api.bilibili.com/x/space/myinfo"

	var userInfo UserInfoResponse
	client := req.C().ImpersonateChrome()
	resp, err := client.R().
		SetHeader("Cookie", cookies).
		Get(apiURL)
	if err != nil {
		return nil, err
	}

	if err := resp.UnmarshalJson(&userInfo); err != nil {
		return nil, fmt.Errorf("解析用户信息失败: %w", err)
	}

	if userInfo.Code == -101 {
		return nil, fmt.Errorf("cookie已失效")
	}

	if userInfo.Code != 0 {
		return nil, fmt.Errorf("获取用户信息失败: %s", userInfo.Message)
	}

	return &userInfo, nil
}

// ValidateCookie 验证Cookie有效性
func ValidateCookie(cookies string) (bool, error) {
	_, err := GetUserInfo(cookies)
	if err != nil {
		if strings.Contains(err.Error(), "已失效") {
			return false, nil
		}
		return false, err
	}
	return true, nil
}

// GetCookieValue 获取Cookie值
func GetCookieValue(cookieStr, key string) string {
	cookies := ParseCookies(cookieStr)
	return cookies[key]
}

// ParseCookies 解析Cookie字符串
func ParseCookies(cookieStr string) map[string]string {
	cookies := make(map[string]string)
	parts := strings.Split(cookieStr, ";")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		kv := strings.SplitN(part, "=", 2)
		if len(kv) == 2 {
			cookies[strings.TrimSpace(kv[0])] = strings.TrimSpace(kv[1])
		}
	}
	return cookies
}

// VideoListResponse 用户投稿视频列表响应
type VideoListResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		List struct {
			Vlist []VideoInfo `json:"vlist"`
		} `json:"list"`
	} `json:"data"`
}

type VideoInfo struct {
	AID    int64  `json:"aid"`
	BVID   string `json:"bvid"`
	Title  string `json:"title"`
	Author string `json:"author"`
	Mid    int64  `json:"mid"`
	Created int64 `json:"created"`
}

// GetUserVideos 获取用户投稿视频列表（带重试）
func (c *BiliClient) GetUserVideos(mid int64, pageSize int) ([]VideoInfo, error) {
	var videos []VideoInfo
	
	err := c.retryWithBackoff(func() error {
		apiURL := fmt.Sprintf("https://api.bilibili.com/x/space/wbi/arc/search?mid=%d&ps=%d&pn=1", mid, pageSize)

		var resp VideoListResponse
		r, err := c.ReqClient.R().
			SetSuccessResult(&resp).
			Get(apiURL)

		if err != nil {
			return fmt.Errorf("获取视频列表失败: %w", err)
		}

		if !r.IsSuccessState() {
			return fmt.Errorf("获取视频列表失败: HTTP %d", r.StatusCode)
		}

		if resp.Code != 0 {
			return fmt.Errorf("获取视频列表失败: %s (code=%d)", resp.Message, resp.Code)
		}

		videos = resp.Data.List.Vlist
		return nil
	})
	
	return videos, err
}

// CommentListResponse 评论列表响应
type CommentListResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		Replies []CommentInfo `json:"replies"`
	} `json:"data"`
}

type CommentInfo struct {
	RPID    int64  `json:"rpid"`
	OID     int64  `json:"oid"`
	Type    int    `json:"type"`
	Mid     int64  `json:"mid"`
	Content struct {
		Message string `json:"message"`
	} `json:"content"`
	Member struct {
		Uname string `json:"uname"`
		Mid   int64  `json:"mid"`
	} `json:"member"`
	CTime int64 `json:"ctime"`
}

// GetVideoComments 获取视频评论（带重试）
func (c *BiliClient) GetVideoComments(oid int64, pageSize int) ([]CommentInfo, error) {
	var comments []CommentInfo
	
	err := c.retryWithBackoff(func() error {
		// type=1 表示视频评论
		apiURL := fmt.Sprintf("https://api.bilibili.com/x/v2/reply?type=1&oid=%d&ps=%d&pn=1&sort=2", oid, pageSize)

		var resp CommentListResponse
		r, err := c.ReqClient.R().
			SetSuccessResult(&resp).
			Get(apiURL)

		if err != nil {
			return fmt.Errorf("获取评论失败: %w", err)
		}

		if !r.IsSuccessState() {
			return fmt.Errorf("获取评论失败: HTTP %d", r.StatusCode)
		}

		if resp.Code != 0 {
			// code=12002 表示评论区已关闭
			if resp.Code == 12002 {
				comments = []CommentInfo{}
				return nil
			}
			return fmt.Errorf("获取评论失败: %s (code=%d)", resp.Message, resp.Code)
		}

		if resp.Data.Replies == nil {
			comments = []CommentInfo{}
			return nil
		}

		comments = resp.Data.Replies
		return nil
	})
	
	return comments, err
}

// ReportCommentRequest 举报评论请求
type ReportCommentRequest struct {
	Type   int    `json:"type"`   // 1=视频评论
	OID    int64  `json:"oid"`    // 视频AID
	RPID   int64  `json:"rpid"`   // 评论ID
	Reason int    `json:"reason"` // 举报理由
	CSRF   string `json:"csrf"`   // CSRF token
}

// ReportCommentResponse 举报评论响应
type ReportCommentResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

// ReportComment 举报评论（带重试）
func (c *BiliClient) ReportComment(oid, rpid int64, reason int) error {
	return c.retryWithBackoff(func() error {
		csrf := GetCookieValue(c.Cookies, "bili_jct")
		if csrf == "" {
			return fmt.Errorf("未找到CSRF token (bili_jct)")
		}

		apiURL := "https://api.bilibili.com/x/v2/reply/report"

		var resp ReportCommentResponse
		r, err := c.ReqClient.R().
			SetFormData(map[string]string{
				"type":   "1",
				"oid":    fmt.Sprintf("%d", oid),
				"rpid":   fmt.Sprintf("%d", rpid),
				"reason": fmt.Sprintf("%d", reason),
				"csrf":   csrf,
			}).
			SetSuccessResult(&resp).
			Post(apiURL)

		if err != nil {
			return fmt.Errorf("举报请求失败: %w", err)
		}

		if !r.IsSuccessState() {
			return fmt.Errorf("举报失败: HTTP %d", r.StatusCode)
		}

		if resp.Code != 0 {
			return fmt.Errorf("举报失败: %s (code=%d)", resp.Message, resp.Code)
		}

		return nil
	})
}

// GetUPInfo 获取UP主信息（带重试）
func (c *BiliClient) GetUPInfo(mid int64) (string, error) {
	var upName string
	
	err := c.retryWithBackoff(func() error {
		apiURL := fmt.Sprintf("https://api.bilibili.com/x/space/acc/info?mid=%d", mid)

		var result struct {
			Code int    `json:"code"`
			Msg  string `json:"message"`
			Data struct {
				Name string `json:"name"`
			} `json:"data"`
		}

		r, err := c.ReqClient.R().
			SetSuccessResult(&result).
			Get(apiURL)

		if err != nil {
			return fmt.Errorf("获取UP主信息失败: %w", err)
		}

		if !r.IsSuccessState() {
			return fmt.Errorf("获取UP主信息失败: HTTP %d", r.StatusCode)
		}

		if result.Code != 0 {
			return fmt.Errorf("获取UP主信息失败: %s (code=%d)", result.Msg, result.Code)
		}

		upName = result.Data.Name
		return nil
	})
	
	return upName, err
}

// QRCodeResponse 二维码响应
type QRCodeResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		URL       string `json:"url"`
		QRcodeKey string `json:"qrcode_key"`
		OauthKey  string `json:"oauthKey"`
		AuthCode  string `json:"auth_code"`
	} `json:"data"`
}

// QRCodePollResponse 轮询响应
type QRCodePollResponse struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    struct {
		URL          string `json:"url"`
		RefreshToken string `json:"refresh_token"`
		Code         int    `json:"code"`
		Message      string `json:"message"`
	} `json:"data"`
	Status bool `json:"status"`
}

// GenerateWebQRCode 生成Web端二维码
func GenerateWebQRCode() (*QRCodeResponse, error) {
	apiURL := "https://passport.bilibili.com/qrcode/getLoginUrl"

	var qrResp QRCodeResponse
	client := req.C().ImpersonateChrome()
	resp, err := client.R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("Referer", "https://www.bilibili.com/").
		Get(apiURL)
	if err != nil {
		return nil, fmt.Errorf("请求二维码失败: %w", err)
	}

	if err := resp.UnmarshalJson(&qrResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if qrResp.Code != 0 {
		return nil, fmt.Errorf("生成二维码失败: %s", qrResp.Message)
	}

	qrResp.Data.AuthCode = qrResp.Data.OauthKey

	return &qrResp, nil
}

// PollWebQRCodeStatus 轮询Web端二维码状态
func PollWebQRCodeStatus(oauthKey string) (*QRCodePollResponse, error) {
	tokenURL := "https://passport.bilibili.com/qrcode/getLoginInfo"

	var pollResp QRCodePollResponse
	client := req.C().ImpersonateChrome()
	resp, err := client.R().
		SetHeader("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36").
		SetHeader("Host", "passport.bilibili.com").
		SetHeader("Referer", "https://passport.bilibili.com/login").
		SetFormData(map[string]string{
			"oauthKey": oauthKey,
			"gourl":    "https://www.bilibili.com/",
		}).
		Post(tokenURL)

	if err != nil {
		return nil, fmt.Errorf("轮询状态失败: %w", err)
	}

	if err := resp.UnmarshalJson(&pollResp); err != nil {
		return nil, fmt.Errorf("解析轮询响应失败: %w", err)
	}

	// 处理状态码
	if pollResp.Status {
		pollResp.Data.Code = 0
	} else {
		var rawData map[string]interface{}
		resp.Unmarshal(&rawData)
		if data, ok := rawData["data"].(map[string]interface{}); ok {
			if codeVal, ok := data["code"].(float64); ok {
				pollResp.Data.Code = int(codeVal)
			}
		}
		if pollResp.Data.Code == 0 {
			pollResp.Data.Code = 86101
		}
	}

	return &pollResp, nil
}

// ExtractCookiesFromWebPollResponse 从Web端轮询响应中提取Cookie
func ExtractCookiesFromWebPollResponse(pollResp *QRCodePollResponse) string {
	if pollResp == nil || pollResp.Data.Code != 0 {
		return ""
	}

	if pollResp.Data.URL == "" {
		return ""
	}

	// 解析URL中的Cookie参数
	parts := strings.Split(pollResp.Data.URL, "?")
	if len(parts) < 2 {
		return ""
	}

	params := make(map[string]string)
	for _, param := range strings.Split(parts[1], "&") {
		kv := strings.SplitN(param, "=", 2)
		if len(kv) == 2 {
			params[kv[0]] = kv[1]
		}
	}

	dedeUserID := params["DedeUserID"]
	sessdata := params["SESSDATA"]
	biliJct := params["bili_jct"]
	dedeUserIDCkMd5 := params["DedeUserID__ckMd5"]
	sid := params["sid"]

	if dedeUserID == "" || sessdata == "" || biliJct == "" {
		return ""
	}

	cookieStrs := []string{
		fmt.Sprintf("bili_jct=%s", biliJct),
		fmt.Sprintf("SESSDATA=%s", sessdata),
		fmt.Sprintf("DedeUserID=%s", dedeUserID),
	}

	if dedeUserIDCkMd5 != "" {
		cookieStrs = append(cookieStrs, fmt.Sprintf("DedeUserID__ckMd5=%s", dedeUserIDCkMd5))
	}
	if sid != "" {
		cookieStrs = append(cookieStrs, fmt.Sprintf("sid=%s", sid))
	}

	return strings.Join(cookieStrs, "; ")
}
