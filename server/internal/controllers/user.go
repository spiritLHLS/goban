package controllers

import (
	"bytes"
	"encoding/base64"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/spiritlhl/goban/internal/bili"
	"github.com/spiritlhl/goban/internal/database"
	"github.com/spiritlhl/goban/internal/models"
	"github.com/spiritlhl/goban/internal/secure"
	"github.com/yeqown/go-qrcode/v2"
	"github.com/yeqown/go-qrcode/writer/standard"
	"gorm.io/gorm"
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
	mu         sync.Mutex
	AuthCode   string
	QRCodeURL  string
	CreateTime int64
	Status     string
	Message    string
}

var loginSessions = make(map[string]*LoginSession)
var loginSessionsMu sync.RWMutex

const sessionExpireTime = 3 * 60

// ListBiliUsers 获取B站用户列表
func ListBiliUsers(c *gin.Context) {
	db := database.GetDB()
	var users []models.BiliUser
	db.Select("id", "created_at", "updated_at", "uid", "uname", "face", "login", "level", "vip_type", "vip_status", "login_time", "expire_time", "cookie_status", "cookie_message", "last_cookie_check").
		Order("created_at DESC").
		Find(&users)

	respondOK(c, users)
}

// LoginUser 生成B站登录二维码
func LoginUser(c *gin.Context) {
	log.Println("开始生成TV端二维码...")

	qrResp, err := bili.GenerateTVQRCode()
	if err != nil {
		log.Printf("生成二维码失败: %v", err)
		respondError(c, http.StatusBadGateway, "生成二维码失败: "+err.Error())
		return
	}

	log.Println("TV端二维码生成成功")

	// 生成二维码图片
	qrc, err := qrcode.NewWith(qrResp.Data.URL,
		qrcode.WithErrorCorrectionLevel(qrcode.ErrorCorrectionMedium),
	)
	if err != nil {
		log.Printf("创建二维码失败: %v", err)
		respondError(c, http.StatusInternalServerError, "创建二维码失败")
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
		respondError(c, http.StatusInternalServerError, "生成PNG失败")
		return
	}

	pngBytes := buf.Bytes()
	log.Printf("[INFO] 生成的PNG大小: %d bytes", len(pngBytes))

	// 验证PNG头部
	if len(pngBytes) < 8 || string(pngBytes[1:4]) != "PNG" {
		log.Printf("[ERROR] PNG格式无效，头部: %v", pngBytes[:min(8, len(pngBytes))])
		respondError(c, http.StatusInternalServerError, "生成的二维码图片格式无效")
		return
	}

	// Base64编码
	imageBase64 := base64.StdEncoding.EncodeToString(pngBytes)
	log.Printf("[INFO] Base64编码长度: %d", len(imageBase64))

	// 使用图片的最后100个字符作为session key
	sessionKey := imageBase64
	if len(imageBase64) > 100 {
		sessionKey = imageBase64[len(imageBase64)-100:]
	}

	setLoginSession(sessionKey, &LoginSession{
		AuthCode:   qrResp.Data.AuthCode,
		QRCodeURL:  qrResp.Data.URL,
		CreateTime: time.Now().Unix(),
		Status:     "pending",
	})

	respondOK(c, gin.H{
		"image": imageBase64,
		"key":   sessionKey,
	})
}

// LoginCheck 检查登录状态（轮询）
func LoginCheck(c *gin.Context) {
	sessionKey := c.Query("key")
	if sessionKey == "" {
		respondOK(c, gin.H{
			"status":  "failed",
			"message": "缺少session key",
		})
		return
	}

	session, exists := getLoginSession(sessionKey)
	if !exists {
		respondOK(c, gin.H{
			"status":  "failed",
			"message": "会话不存在",
		})
		return
	}

	session.mu.Lock()
	defer session.mu.Unlock()

	// 检查会话是否过期
	if time.Now().Unix()-session.CreateTime > sessionExpireTime {
		session.Status = "expired"
		session.Message = "二维码已过期"
		deleteLoginSession(sessionKey)
		respondOK(c, gin.H{
			"status":  "expired",
			"message": "二维码已过期，请重新获取",
		})
		return
	}

	if session.Status != "pending" {
		if session.Status == "success" || session.Status == "failed" {
			deleteLoginSession(sessionKey)
		}
		respondOK(c, gin.H{
			"status":  session.Status,
			"message": session.Message,
		})
		return
	}

	// 轮询登录状态
	pollResp, err := bili.PollTVQRCodeStatus(session.AuthCode)
	if err != nil {
		log.Printf("[ERROR] 轮询失败: %v", err)
		respondOK(c, gin.H{
			"status":  "pending",
			"message": "检查中...",
		})
		return
	}

	log.Printf("[POLL] 轮询响应 - code: %d, message: %s",
		pollResp.Data.Code, pollResp.Message)

	switch pollResp.Data.Code {
	case 0: // 登录成功
		cookieStr := bili.ExtractCookiesFromTVPollResponse(pollResp)
		log.Printf("[TV] 提取到Cookie，长度: %d", len(cookieStr))

		if cookieStr == "" {
			session.Status = "failed"
			session.Message = "获取Cookie失败"
			respondOK(c, gin.H{
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
			respondOK(c, gin.H{
				"status":  "failed",
				"message": "获取用户信息失败: " + err.Error(),
			})
			return
		}

		user, err := saveBiliUserWithCookies(cookieStr, userInfo)
		if err != nil {
			session.Status = "failed"
			session.Message = "保存用户失败"
			respondOK(c, gin.H{
				"status":  "failed",
				"message": "保存用户失败: " + err.Error(),
			})
			return
		}

		session.Status = "success"
		session.Message = "登录成功"

		log.Printf("[INFO] 用户登录成功: UID=%d, Uname=%s", user.UID, user.Uname)

		respondOK(c, gin.H{
			"status":  "success",
			"message": "登录成功",
		})

	case 86038: // 二维码已失效
		session.Status = "expired"
		session.Message = "二维码已过期"
		respondOK(c, gin.H{
			"status":  "expired",
			"message": "二维码已过期，请重新获取",
		})

	case 86090: // 已扫码未确认
		respondOK(c, gin.H{
			"status":  "scanned",
			"message": "已扫码，等待确认...",
		})

	case 86101: // 等待扫码
		respondOK(c, gin.H{
			"status":  "pending",
			"message": "等待扫码...",
		})

	default:
		respondOK(c, gin.H{
			"status":  "pending",
			"message": "检查中...",
		})
	}
}

// LoginCancel 取消登录
func LoginCancel(c *gin.Context) {
	sessionKey := c.Query("key")
	if sessionKey != "" {
		deleteLoginSession(sessionKey)
	}
	respondCreated(c, "已取消", gin.H{"message": "已取消"})
}

// LoginByCookie 通过Cookie直接登录
func LoginByCookie(c *gin.Context) {
	var req struct {
		Cookies string `json:"cookies" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		respondError(c, http.StatusBadRequest, "请求参数错误")
		return
	}

	cookieStr := strings.TrimSpace(req.Cookies)
	if cookieStr == "" {
		respondError(c, http.StatusBadRequest, "Cookie不能为空")
		return
	}

	// 验证Cookie
	valid, err := bili.ValidateCookie(cookieStr)
	if err != nil {
		respondError(c, http.StatusBadGateway, "验证Cookie失败: "+err.Error())
		return
	}

	if !valid {
		respondError(c, http.StatusUnauthorized, "Cookie已失效或格式错误")
		return
	}

	// 获取用户信息
	userInfo, err := bili.GetUserInfo(cookieStr)
	if err != nil {
		respondError(c, http.StatusBadGateway, "获取用户信息失败")
		return
	}

	user, err := saveBiliUserWithCookies(cookieStr, userInfo)
	if err != nil {
		respondError(c, http.StatusInternalServerError, "保存用户失败: "+err.Error())
		return
	}

	log.Printf("[INFO] 用户通过Cookie登录成功: UID=%d, Uname=%s", user.UID, user.Uname)

	respondCreated(c, "登录成功", gin.H{
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
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}
	if !requireDeleteConfirmation(c, user.Uname, strconv.FormatInt(user.UID, 10)) {
		return
	}

	if err := db.Transaction(func(tx *gorm.DB) error {
		var tasks []models.MonitorTask
		if err := tx.Where("user_id = ?", user.ID).Find(&tasks).Error; err != nil {
			return err
		}
		for _, task := range tasks {
			if err := tx.Where("task_id = ?", task.ID).Delete(&models.MonitorTarget{}).Error; err != nil {
				return err
			}
			if err := tx.Where("task_id = ?", task.ID).Delete(&models.MonitorLog{}).Error; err != nil {
				return err
			}
			if err := tx.Where("task_id = ?", task.ID).Delete(&models.ReportRecord{}).Error; err != nil {
				return err
			}
		}
		if err := tx.Where("user_id = ?", user.ID).Delete(&models.MonitorTask{}).Error; err != nil {
			return err
		}
		return tx.Delete(&user).Error
	}); err != nil {
		respondError(c, http.StatusInternalServerError, "删除失败: "+err.Error())
		return
	}

	var remaining int64
	if err := db.Model(&models.BiliUser{}).Where("id = ?", user.ID).Count(&remaining).Error; err != nil || remaining != 0 {
		respondError(c, http.StatusInternalServerError, "删除结果校验失败")
		return
	}

	respondCreated(c, "删除成功", gin.H{"type": "success", "msg": "删除成功", "deleted_id": user.ID})
}

// CheckBiliUserCookie 手动检测账号Cookie有效性
func CheckBiliUserCookie(c *gin.Context) {
	db := database.GetDB()
	var user models.BiliUser
	if err := db.First(&user, c.Param("id")).Error; err != nil {
		respondError(c, http.StatusNotFound, "用户不存在")
		return
	}

	cookies, err := secure.DecryptString(user.Cookies)
	now := time.Now()
	if err != nil {
		db.Model(&user).Updates(map[string]interface{}{
			"login":             false,
			"cookie_status":     "invalid",
			"cookie_message":    "Cookie解密失败: " + err.Error(),
			"last_cookie_check": now,
		})
		respondError(c, http.StatusConflict, "Cookie解密失败")
		return
	}

	valid, err := bili.ValidateCookie(cookies)
	updates := map[string]interface{}{
		"last_cookie_check": now,
	}
	if err != nil {
		updates["cookie_status"] = "unknown"
		updates["cookie_message"] = err.Error()
		db.Model(&user).Updates(updates)
		respondError(c, http.StatusBadGateway, "检测失败: "+err.Error())
		return
	}
	if valid {
		updates["login"] = true
		updates["cookie_status"] = "valid"
		updates["cookie_message"] = "Cookie有效"
		db.Model(&user).Updates(updates)
		respondOK(c, gin.H{"type": "success", "msg": "Cookie有效"})
		return
	}

	updates["login"] = false
	updates["cookie_status"] = "invalid"
	updates["cookie_message"] = "Cookie已失效"
	db.Model(&user).Updates(updates)
	respondOK(c, gin.H{"type": "error", "msg": "Cookie已失效"})
}

func getLoginSession(key string) (*LoginSession, bool) {
	loginSessionsMu.RLock()
	defer loginSessionsMu.RUnlock()
	session, ok := loginSessions[key]
	return session, ok
}

func setLoginSession(key string, session *LoginSession) {
	loginSessionsMu.Lock()
	defer loginSessionsMu.Unlock()
	loginSessions[key] = session
}

func deleteLoginSession(key string) {
	loginSessionsMu.Lock()
	defer loginSessionsMu.Unlock()
	delete(loginSessions, key)
}

func saveBiliUserWithCookies(cookieStr string, userInfo *bili.UserInfoResponse) (models.BiliUser, error) {
	encryptedCookies, err := secure.EncryptString(cookieStr)
	if err != nil {
		return models.BiliUser{}, err
	}

	db := database.GetDB()
	var user models.BiliUser
	now := time.Now()
	expireTime := now.Add(30 * 24 * time.Hour)

	result := db.Where("uid = ?", userInfo.Data.Mid).First(&user)
	if result.Error != nil {
		user = models.BiliUser{
			UID:             userInfo.Data.Mid,
			Uname:           userInfo.Data.Uname,
			Face:            userInfo.Data.Face,
			Cookies:         encryptedCookies,
			Login:           true,
			Level:           userInfo.GetLevel(),
			LoginTime:       now,
			ExpireTime:      expireTime,
			CookieStatus:    "valid",
			CookieMessage:   "登录成功",
			LastCookieCheck: &now,
		}
		if err := db.Create(&user).Error; err != nil {
			return models.BiliUser{}, err
		}
	} else {
		user.Uname = userInfo.Data.Uname
		user.Face = userInfo.Data.Face
		user.Cookies = encryptedCookies
		user.Login = true
		user.Level = userInfo.GetLevel()
		user.LoginTime = now
		user.ExpireTime = expireTime
		user.CookieStatus = "valid"
		user.CookieMessage = "登录成功"
		user.LastCookieCheck = &now
		if err := db.Save(&user).Error; err != nil {
			return models.BiliUser{}, err
		}
	}

	user.Cookies = ""
	return user, nil
}
