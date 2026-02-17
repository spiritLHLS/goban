package controllers

import (
	"bytes"
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/imroc/req/v3"
	"github.com/spiritlhl/goban/internal/bili"
	"github.com/spiritlhl/goban/internal/database"
	"github.com/spiritlhl/goban/internal/models"
	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
)

// nopCloser 包装 io.Writer 为 io.WriteCloser
type nopCloser struct {
	io.Writer
}

func (nopCloser) Close() error { return nil }

// min 辅助函数
func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// LoginSession 登录会话
type LoginSession struct {
	AuthCode   string
	QRCodeURL  string
	CreateTime int64
	Status     string
	Message    string
}

var loginSessions = make(map[string]*LoginSession)

const sessionExpireTime = 3 * 60

// ListBiliUsers 获取B站用户列表
func ListBiliUsers(c *gin.Context) {
	db := database.GetDB()
	var users []models.BiliUser
	db.Select("id", "created_at", "updated_at", "uid", "uname", "face", "login", "level", "vip_type", "vip_status", "login_time", "expire_time").
		Order("created_at DESC").
		Find(&users)

	c.JSON(http.StatusOK, users)
}

// LoginUser 生成B站登录二维码
func LoginUser(c *gin.Context) {
	log.Println("开始生成Web端二维码...")

	qrResp, err := bili.GenerateWebQRCode()
	if err != nil {
		log.Printf("生成二维码失败: %v", err)
		c.JSON(http.StatusOK, gin.H{"error": "生成二维码失败: " + err.Error()})
		return
	}

	log.Printf("Web端二维码URL: %s, AuthCode: %s", qrResp.Data.URL, qrResp.Data.AuthCode)

	// 生成二维码图片
	qrc, err := qrcode.NewWith(qrResp.Data.URL,
		qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionMedium),
	)
	if err != nil {
		log.Printf("创建二维码失败: %v", err)
		c.JSON(http.StatusOK, gin.H{"error": "创建二维码失败"})
		return
	}

	buf := new(bytes.Buffer)
	w := nopCloser{buf}
	stdWriter := standard.NewWithWriter(w,
		standard.WithQRWidth(10),
		standard.WithBuiltinImageEncoder(standard.PNG_FORMAT),
	)
	if err = qrc.Save(stdWriter); err != nil {
		log.Printf("生成PNG失败: %v", err)
		c.JSON(http.StatusOK, gin.H{"error": "生成PNG失败"})
		return
	}

	pngBytes := buf.Bytes()
	log.Printf("[INFO] 生成的PNG大小: %d bytes", len(pngBytes))

	// 验证PNG头部
	if len(pngBytes) < 8 || string(pngBytes[1:4]) != "PNG" {
		log.Printf("[ERROR] PNG格式无效，头部: %v", pngBytes[:min(8, len(pngBytes))])
		c.JSON(http.StatusOK, gin.H{"error": "生成的二维码图片格式无效"})
		return
	}

	// Base64编码
	imageBase64 := base64.StdEncoding.EncodeToString(pngBytes)
	log.Printf("[INFO] Base64编码长度: %d", len(imageBase64))

	// 使用图片的最后100个字符作为session key
	sessionKey := imageBase64[len(imageBase64)-100:]

	loginSessions[sessionKey] = &LoginSession{
		AuthCode:   qrResp.Data.AuthCode,
		QRCodeURL:  qrResp.Data.URL,
		CreateTime: time.Now().Unix(),
		Status:     "pending",
	}

	c.JSON(http.StatusOK, gin.H{
		"image": imageBase64,
		"key":   sessionKey,
	})
}

// LoginCheck 检查登录状态（轮询）
func LoginCheck(c *gin.Context) {
	sessionKey := c.Query("key")
	if sessionKey == "" {
		c.JSON(http.StatusOK, gin.H{
			"status":  "failed",
			"message": "缺少session key",
		})
		return
	}

	session, exists := loginSessions[sessionKey]
	if !exists {
		c.JSON(http.StatusOK, gin.H{
			"status":  "failed",
			"message": "会话不存在",
		})
		return
	}

	// 检查会话是否过期
	if time.Now().Unix()-session.CreateTime > sessionExpireTime {
		session.Status = "expired"
		session.Message = "二维码已过期"
		delete(loginSessions, sessionKey)
		c.JSON(http.StatusOK, gin.H{
			"status":  "expired",
			"message": "二维码已过期，请重新获取",
		})
		return
	}

	if session.Status != "pending" {
		if session.Status == "success" || session.Status == "failed" {
			delete(loginSessions, sessionKey)
		}
		c.JSON(http.StatusOK, gin.H{
			"status":  session.Status,
			"message": session.Message,
		})
		return
	}

	// 轮询登录状态
	pollResp, err := bili.PollWebQRCodeStatus(session.AuthCode)
	if err != nil {
		log.Printf("[ERROR] 轮询失败: %v", err)
		c.JSON(http.StatusOK, gin.H{
			"status":  "pending",
			"message": "检查中...",
		})
		return
	}

	log.Printf("[POLL] 轮询响应 - code: %d", pollResp.Data.Code)

	switch pollResp.Data.Code {
	case 0: // 登录成功
		cookieStr := bili.ExtractCookiesFromWebPollResponse(pollResp)
		log.Printf("[Web] 提取到的Cookie: %s", cookieStr)
		
		if cookieStr == "" {
			session.Status = "failed"
			session.Message = "获取Cookie失败"
			c.JSON(http.StatusOK, gin.H{
				"status":  "failed",
				"message": "获取Cookie失败",
			})
			return
		}

		// 获取用户信息
		userInfo, err := bili.GetUserInfo(cookieStr)
		if err != nil {
			session.Status = "failed"
			session.Message = "获取用户信息失败"
			c.JSON(http.StatusOK, gin.H{
				"status":  "failed",
				"message": "获取用户信息失败: " + err.Error(),
			})
			return
		}

		// 保存用户到数据库
		db := database.GetDB()
		var user models.BiliUser

		now := time.Now()
		expireTime := now.Add(30 * 24 * time.Hour)

		result := db.Where("uid = ?", userInfo.Data.Mid).First(&user)
		if result.Error != nil {
			// 新用户
			user = models.BiliUser{
				UID:        userInfo.Data.Mid,
				Uname:      userInfo.Data.Uname,
				Face:       userInfo.Data.Face,
				Cookies:    cookieStr,
				Login:      true,
				Level:      userInfo.Data.Level,
				LoginTime:  now,
				ExpireTime: expireTime,
			}
			db.Create(&user)
		} else {
			// 更新用户
			user.Uname = userInfo.Data.Uname
			user.Face = userInfo.Data.Face
			user.Cookies = cookieStr
			user.Login = true
			user.Level = userInfo.Data.Level
			user.LoginTime = now
			user.ExpireTime = expireTime
			db.Save(&user)
		}

		session.Status = "success"
		session.Message = "登录成功"

		log.Printf("[INFO] 用户登录成功: UID=%d, Uname=%s", user.UID, user.Uname)

		c.JSON(http.StatusOK, gin.H{
			"status":  "success",
			"message": "登录成功",
		})

	case 86038: // 二维码已失效
		session.Status = "expired"
		session.Message = "二维码已过期"
		c.JSON(http.StatusOK, gin.H{
			"status":  "expired",
			"message": "二维码已过期，请重新获取",
		})

	case 86090: // 已扫码未确认
		c.JSON(http.StatusOK, gin.H{
			"status":  "scanned",
			"message": "已扫码，等待确认...",
		})

	case 86101: // 等待扫码
		c.JSON(http.StatusOK, gin.H{
			"status":  "pending",
			"message": "等待扫码...",
		})

	default:
		c.JSON(http.StatusOK, gin.H{
			"status":  "pending",
			"message": "检查中...",
		})
	}
}

// LoginCancel 取消登录
func LoginCancel(c *gin.Context) {
	sessionKey := c.Query("key")
	if sessionKey != "" {
		delete(loginSessions, sessionKey)
	}
	c.JSON(http.StatusOK, gin.H{"message": "已取消"})
}

// LoginByCookie 通过Cookie直接登录
func LoginByCookie(c *gin.Context) {
	var req struct {
		Cookies string `json:"cookies" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"type": "error", "msg": "请求参数错误"})
		return
	}

	cookieStr := strings.TrimSpace(req.Cookies)
	if cookieStr == "" {
		c.JSON(http.StatusOK, gin.H{"type": "error", "msg": "Cookie不能为空"})
		return
	}

	// 验证Cookie
	valid, err := bili.ValidateCookie(cookieStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"type": "error", "msg": "验证Cookie失败: " + err.Error()})
		return
	}

	if !valid {
		c.JSON(http.StatusOK, gin.H{"type": "error", "msg": "Cookie已失效或格式错误"})
		return
	}

	// 获取用户信息
	userInfo, err := bili.GetUserInfo(cookieStr)
	if err != nil {
		c.JSON(http.StatusOK, gin.H{"type": "error", "msg": "获取用户信息失败"})
		return
	}

	// 保存用户
	db := database.GetDB()
	var user models.BiliUser

	now := time.Now()
	expireTime := now.Add(30 * 24 * time.Hour)

	result := db.Where("uid = ?", userInfo.Data.Mid).First(&user)
	if result.Error != nil {
		user = models.BiliUser{
			UID:        userInfo.Data.Mid,
			Uname:      userInfo.Data.Uname,
			Face:       userInfo.Data.Face,
			Cookies:    cookieStr,
			Login:      true,
			Level:      userInfo.Data.Level,
			LoginTime:  now,
			ExpireTime: expireTime,
		}
		db.Create(&user)
	} else {
		user.Uname = userInfo.Data.Uname
		user.Face = userInfo.Data.Face
		user.Cookies = cookieStr
		user.Login = true
		user.Level = userInfo.Data.Level
		user.LoginTime = now
		user.ExpireTime = expireTime
		db.Save(&user)
	}

	log.Printf("[INFO] 用户通过Cookie登录成功: UID=%d, Uname=%s", user.UID, user.Uname)

	c.JSON(http.StatusOK, gin.H{
		"type": "success",
		"msg":  "登录成功",
		"user": user,
	})
}

// DeleteBiliUser 删除B站用户
func DeleteBiliUser(c *gin.Context) {
	id := c.Param("id")
	db := database.GetDB()

	var user models.BiliUser
	if err := db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusOK, gin.H{"type": "error", "msg": "用户不存在"})
		return
	}

	// 删除关联的监控任务
	db.Where("user_id = ?", user.ID).Delete(&models.MonitorTask{})

	// 删除用户
	db.Delete(&user)

	c.JSON(http.StatusOK, gin.H{"type": "success", "msg": "删除成功"})
}
