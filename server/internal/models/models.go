package models

import (
	"time"
)

// BiliUser B站用户信息
type BiliUser struct {
	ID         uint      `json:"id" gorm:"primaryKey"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	UID        int64     `json:"uid" gorm:"uniqueIndex"`
	Uname      string    `json:"uname"`
	Face       string    `json:"face"`
	Cookies    string    `json:"-"` // 不返回给前端
	Login      bool      `json:"login"`
	Level      int       `json:"level"`
	VipType    int       `json:"vip_type"`
	VipStatus  int       `json:"vip_status"`
	LoginTime  time.Time `json:"login_time"`
	ExpireTime time.Time `json:"expire_time"`
}

// MonitorTask 监控任务
type MonitorTask struct {
	ID            uint      `json:"id" gorm:"primaryKey"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`
	UserID        uint      `json:"user_id"`                                   // 关联的B站用户ID
	User          BiliUser  `json:"user" gorm:"foreignKey:UserID"`             // 关联的B站用户
	TargetUID     int64     `json:"target_uid"`                                // 要监控的UP主UID
	TargetUname   string    `json:"target_uname"`                              // 要监控的UP主名称
	VideoCount    int       `json:"video_count" gorm:"default:5"`              // 监控最新多少条视频
	CommentCount  int       `json:"comment_count" gorm:"default:50"`           // 监控每个视频的多少条评论
	Keywords      string    `json:"keywords"`                                  // 关键字，逗号分隔
	Enabled       bool      `json:"enabled" gorm:"default:true"`               // 是否启用
	Interval      int       `json:"interval" gorm:"default:300"`               // 监控间隔（秒）
	ReportDelay   int       `json:"report_delay" gorm:"default:6"`             // 举报间隔（秒），默认6秒防止触发1分钟10次限制
	MaxRetries    int       `json:"max_retries" gorm:"default:3"`             // API最大重试次数
	RetryInterval int       `json:"retry_interval" gorm:"default:2"`           // API重试基础间隔（秒），使用指数退避
	ProxyURL      string    `json:"proxy_url"`                                 // 代理地址，如 http://proxy:port 或 socks5://proxy:port
	LastCheck     time.Time `json:"last_check"`                                // 上次检查时间
}

// MonitorLog 监控日志
type MonitorLog struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"created_at"`
	TaskID    uint      `json:"task_id"`
	Task      MonitorTask `json:"task" gorm:"foreignKey:TaskID"`
	Message   string    `json:"message"`
	Level     string    `json:"level"` // info, warning, error
}

// ReportRecord 举报记录
type ReportRecord struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	TaskID    uint      `json:"task_id"`
	Task      MonitorTask `json:"task" gorm:"foreignKey:TaskID"`
	AVID      int64     `json:"avid"`      // 视频AV号
	BVID      string    `json:"bvid"`      // 视频BV号
	VideoTitle string   `json:"video_title"` // 视频标题
	CommentID int64     `json:"comment_id"` // 评论ID
	CommentContent string `json:"comment_content"` // 评论内容
	CommentUser string `json:"comment_user"` // 评论用户
	MatchedKeyword string `json:"matched_keyword"` // 匹配的关键字
	Reason     int      `json:"reason" gorm:"default:11"` // 举报理由：11=传谣类
	Success    bool     `json:"success"` // 举报是否成功
	Message    string   `json:"message"` // 举报结果消息
}
