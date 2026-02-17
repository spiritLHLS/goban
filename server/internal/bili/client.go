package bili

import (
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"net/url"
	"sort"
	"strings"
	"time"

	"github.com/imroc/req/v3"
)

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

const (
	AppKey    = "4409e2ce8ffd12b8"
	AppSecret = "59b43e04ad6965f34319062b478f83dd"
)

// signParams 对参数进行签名（TV端APP登录需要）
func signParams(params map[string]string) map[string]string {
	// 按key排序
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 构建查询字符串
	var query strings.Builder
	for _, k := range keys {
		if query.Len() > 0 {
			query.WriteString("&")
		}
		query.WriteString(k)
		query.WriteString("=")
		query.WriteString(params[k])
	}

	// 添加AppSecret并计算MD5
	query.WriteString(AppSecret)
	hash := md5.Sum([]byte(query.String()))
	sign := hex.EncodeToString(hash[:])

	params["sign"] = sign
	return params
}

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
		// TV端返回的cookie信息
		CookieInfo struct {
			Cookies []struct {
				Name     string `json:"name"`
				Value    string `json:"value"`
				HttpOnly int    `json:"http_only"`
				Expires  int64  `json:"expires"`
				Secure   int    `json:"secure"`
			} `json:"cookies"`
		} `json:"cookie_info"`
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

// GenerateTVQRCode 生成TV端二维码
func GenerateTVQRCode() (*QRCodeResponse, error) {
	params := map[string]string{
		"appkey":   AppKey,
		"local_id": "0",
		"ts":       "0",
	}

	params = signParams(params)
	apiURL := "https://passport.bilibili.com/x/passport-tv-login/qrcode/auth_code"

	log.Printf("[TV_QR] 请求URL: %s", apiURL)
	log.Printf("[TV_QR] 请求参数: appkey=%s, local_id=%s, ts=%s, sign=%s",
		params["appkey"], params["local_id"], params["ts"], params["sign"])

	var qrResp QRCodeResponse
	client := req.C().ImpersonateChrome()
	resp, err := client.R().
		SetFormDataFromValues(url.Values{
			"appkey":   {params["appkey"]},
			"local_id": {params["local_id"]},
			"ts":       {params["ts"]},
			"sign":     {params["sign"]},
		}).
		Post(apiURL)
	if err != nil {
		return nil, fmt.Errorf("请求二维码失败: %w", err)
	}

	rawBody := resp.String()
	log.Printf("[TV_QR_DEBUG] 原始响应: %s", rawBody)

	if err := resp.UnmarshalJson(&qrResp); err != nil {
		return nil, fmt.Errorf("解析响应失败: %w", err)
	}

	if qrResp.Code != 0 {
		return nil, fmt.Errorf("生成TV端二维码失败 code=%d msg=%s", qrResp.Code, qrResp.Message)
	}

	log.Printf("[TV_QR] 生成成功 - url: %s, auth_code: %s", qrResp.Data.URL, qrResp.Data.AuthCode)

	return &qrResp, nil
}

// PollTVQRCodeStatus 轮询TV端二维码状态
func PollTVQRCodeStatus(authCode string) (*QRCodePollResponse, error) {
	params := map[string]string{
		"appkey":    AppKey,
		"auth_code": authCode,
		"local_id":  "0",
		"ts":        "0",
	}

	params = signParams(params)
	apiURL := "https://passport.bilibili.com/x/passport-tv-login/qrcode/poll"

	log.Printf("[TV_POLL] 轮询 authCode: %s", authCode)

	var pollResp QRCodePollResponse
	client := req.C().ImpersonateChrome()
	resp, err := client.R().
		SetFormDataFromValues(url.Values{
			"appkey":    {params["appkey"]},
			"auth_code": {params["auth_code"]},
			"local_id":  {params["local_id"]},
			"ts":        {params["ts"]},
			"sign":      {params["sign"]},
		}).
		Post(apiURL)
	if err != nil {
		return nil, fmt.Errorf("轮询状态失败: %w", err)
	}

	rawBody := resp.String()
	log.Printf("[TV_POLL_DEBUG] 原始响应: %s", rawBody)

	if err := resp.UnmarshalJson(&pollResp); err != nil {
		return nil, fmt.Errorf("解析轮询响应失败: %w", err)
	}

	log.Printf("[TV_POLL] 原始响应 - code=%d, message=%s, data.code=%d",
		pollResp.Code, pollResp.Message, pollResp.Data.Code)

	// TV端的状态码在顶层code字段
	if pollResp.Code == 0 {
		// 登录成功
		pollResp.Data.Code = 0
		log.Printf("[TV_POLL] 登录成功")
	} else {
		// 将顶层code映射到data.code以保持统一接口
		pollResp.Data.Code = pollResp.Code
		switch pollResp.Code {
		case 86038:
			log.Printf("[TV_POLL] 二维码已过期")
		case 86090:
			log.Printf("[TV_POLL] 已扫码，等待确认")
		case 86101, 86039:
			// 86039: 二维码尚未确认 (未扫码)
			// 86101: 未扫码
			pollResp.Data.Code = 86101
			log.Printf("[TV_POLL] 等待扫码")
		default:
			log.Printf("[TV_POLL] 未知状态 code=%d", pollResp.Code)
		}
	}

	return &pollResp, nil
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

	// 打印原始响应用于调试
	rawBody := resp.String()
	log.Printf("[WEB_POLL_DEBUG] 原始响应: %s", rawBody)

	if err := resp.UnmarshalJson(&pollResp); err != nil {
		return nil, fmt.Errorf("解析轮询响应失败: %w", err)
	}

	// 参考 gobup 项目的状态码处理逻辑：
	// status=true: 登录成功
	// data.code=-4: 未扫码
	// data.code=-5: 已扫码待确认
	// data.code=-2: 二维码已过期
	log.Printf("[WEB_POLL] 原始响应 - status: %v, data.code: %d, data.message: %s",
		pollResp.Status, pollResp.Data.Code, pollResp.Data.Message)

	// 优先判断 status 字段
	if pollResp.Status {
		// 登录成功
		pollResp.Data.Code = 0
		log.Printf("[WEB_POLL] 登录成功 - url=%s", pollResp.Data.URL)
	} else {
		// 根据 data.code 字段判断状态
		switch pollResp.Data.Code {
		case -4:
			// 二维码未失效，等待扫码
			pollResp.Data.Code = 86101
			log.Printf("[WEB_POLL] 等待扫码")
		case -5:
			// 已扫码，等待确认
			pollResp.Data.Code = 86090
			log.Printf("[WEB_POLL] 已扫码，等待确认")
		case -2:
			// 二维码已失效
			pollResp.Data.Code = 86038
			log.Printf("[WEB_POLL] 二维码已过期")
		default:
			// 其他未知状态，默认为等待扫码
			pollResp.Data.Code = 86101
			log.Printf("[WEB_POLL] 未知状态 code=%d，默认等待扫码", pollResp.Data.Code)
		}
	}

	return &pollResp, nil
}

// ExtractCookiesFromWebPollResponse 从Web端轮询响应中提取Cookie
func ExtractCookiesFromWebPollResponse(pollResp *QRCodePollResponse, client *req.Client) string {
	if pollResp == nil || pollResp.Data.Code != 0 {
		log.Printf("[WEB_COOKIE] 登录未完成，跳过Cookie提取 - code=%d", pollResp.Data.Code)
		return ""
	}

	if pollResp.Data.URL == "" {
		log.Printf("[WEB_COOKIE] 错误：URL为空")
		return ""
	}

	// Web端登录成功后，URL中包含Cookie参数
	log.Printf("[WEB_COOKIE] 解析登录URL: %s", pollResp.Data.URL[:min(100, len(pollResp.Data.URL))])

	// 解析URL中的Cookie参数
	parts := strings.Split(pollResp.Data.URL, "?")
	if len(parts) < 2 {
		log.Printf("[WEB_COOKIE] URL没有查询参数")
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
		log.Printf("[WEB_COOKIE] 关键字段缺失 - DedeUserID: %v, SESSDATA: %v, bili_jct: %v",
			dedeUserID != "", sessdata != "", biliJct != "")
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

	result := strings.Join(cookieStrs, "; ")
	log.Printf("[WEB_COOKIE] 提取成功 - DedeUserID: %s, SESSDATA长度: %d, bili_jct长度: %d",
		dedeUserID, len(sessdata), len(biliJct))

	return result
}

// ExtractCookiesFromTVPollResponse 从TV端轮询响应中提取Cookie
func ExtractCookiesFromTVPollResponse(pollResp *QRCodePollResponse) string {
	if pollResp == nil || pollResp.Data.Code != 0 {
		log.Printf("[TV_COOKIE] 登录未完成，跳过Cookie提取 - code=%d", pollResp.Data.Code)
		return ""
	}

	// TV端登录成功后，从 cookie_info.cookies 数组中提取
	if len(pollResp.Data.CookieInfo.Cookies) == 0 {
		log.Printf("[TV_COOKIE] 错误：cookie_info.cookies为空")
		return ""
	}

	// 构建 Cookie 字符串
	cookieMap := make(map[string]string)
	for _, cookie := range pollResp.Data.CookieInfo.Cookies {
		cookieMap[cookie.Name] = cookie.Value
	}

	// 按顺序构建 Cookie（参考gobup项目）
	cookieStrs := []string{
		fmt.Sprintf("bili_jct=%s", cookieMap["bili_jct"]),
		fmt.Sprintf("SESSDATA=%s", cookieMap["SESSDATA"]),
		fmt.Sprintf("DedeUserID=%s", cookieMap["DedeUserID"]),
	}

	if val, ok := cookieMap["DedeUserID__ckMd5"]; ok && val != "" {
		cookieStrs = append(cookieStrs, fmt.Sprintf("DedeUserID__ckMd5=%s", val))
	}
	if val, ok := cookieMap["sid"]; ok && val != "" {
		cookieStrs = append(cookieStrs, fmt.Sprintf("sid=%s", val))
	}

	result := strings.Join(cookieStrs, "; ")
	log.Printf("[TV_COOKIE] 提取成功 - DedeUserID: %s, SESSDATA长度: %d",
		cookieMap["DedeUserID"], len(cookieMap["SESSDATA"]))

	return result
}
