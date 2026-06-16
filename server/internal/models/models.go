package models

import (
	"time"
)

// BiliUser B站用户信息
type BiliUser struct {
	ID              uint       `json:"id" gorm:"primaryKey"`
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	UID             int64      `json:"uid" gorm:"uniqueIndex"`
	Uname           string     `json:"uname"`
	Face            string     `json:"face"`
	Cookies         string     `json:"-"` // 加密存储，不返回给前端
	Login           bool       `json:"login"`
	Level           int        `json:"level"`
	VipType         int        `json:"vip_type"`
	VipStatus       int        `json:"vip_status"`
	LoginTime       time.Time  `json:"login_time"`
	ExpireTime      time.Time  `json:"expire_time"`
	CookieStatus    string     `json:"cookie_status" gorm:"default:unknown"`
	CookieMessage   string     `json:"cookie_message"`
	LastCookieCheck *time.Time `json:"last_cookie_check"`
}

// MonitorTask 监控任务
type MonitorTask struct {
	ID               uint            `json:"id" gorm:"primaryKey"`
	CreatedAt        time.Time       `json:"created_at"`
	UpdatedAt        time.Time       `json:"updated_at"`
	UserID           uint            `json:"user_id"`                       // 关联的B站用户ID
	User             BiliUser        `json:"user" gorm:"foreignKey:UserID"` // 关联的B站用户
	Name             string          `json:"name"`                          // 任务名称
	Targets          []MonitorTarget `json:"targets" gorm:"foreignKey:TaskID;constraint:OnDelete:CASCADE;"`
	VideoCount       int             `json:"video_count" gorm:"default:5"`          // 监控最新多少条视频
	CommentCount     int             `json:"comment_count" gorm:"default:50"`       // 监控每个视频的多少条评论
	Keywords         string          `json:"keywords"`                              // 兼容的临时关键字，逗号或换行分隔
	KeywordRuleIDs   string          `json:"keyword_rule_ids"`                      // 关联的关键字规则ID，逗号分隔；为空表示使用所有启用规则
	Enabled          bool            `json:"enabled" gorm:"default:true"`           // 是否启用
	Interval         int             `json:"interval" gorm:"default:300"`           // 监控间隔（秒）
	ReportDelay      int             `json:"report_delay" gorm:"default:30"`        // 举报间隔（秒）
	DailyReportLimit int             `json:"daily_report_limit" gorm:"default:100"` // 单账号每日成功举报上限
	MaxRetries       int             `json:"max_retries" gorm:"default:3"`          // API最大重试次数
	RetryInterval    int             `json:"retry_interval" gorm:"default:2"`       // API重试基础间隔（秒），使用指数退避
	ProxyURL         string          `json:"proxy_url"`                             // 代理地址，如 http://proxy:port 或 socks5://proxy:port
	LastCheck        time.Time       `json:"last_check"`                            // 上次检查时间
	LastSuccessAt    *time.Time      `json:"last_success_at"`
	LastStatus       string          `json:"last_status"`
	LastError        string          `json:"last_error"`
	NextRunAt        *time.Time      `json:"next_run_at"`
	BackoffUntil     *time.Time      `json:"backoff_until"`
	BackoffReason    string          `json:"backoff_reason"`
	BackoffAttempt   int             `json:"backoff_attempt"`
	ProgressTotal    int64           `json:"progress_total"`
	ProgressDone     int64           `json:"progress_done"`
	ProgressMessage  string          `json:"progress_message"`
	CheckedComments  int64           `json:"checked_comments"`
	MatchedComments  int64           `json:"matched_comments"`
	ReportCount      int64           `json:"report_count"`
}

// MonitorTarget 单个监控任务下的UP主目标
type MonitorTarget struct {
	ID              uint      `json:"id" gorm:"primaryKey"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	TaskID          uint      `json:"task_id" gorm:"index"`
	UID             int64     `json:"uid" gorm:"index"`
	Uname           string    `json:"uname"`
	LastCheck       time.Time `json:"last_check"`
	LastStatus      string    `json:"last_status"`
	LastError       string    `json:"last_error"`
	CheckedComments int64     `json:"checked_comments"`
	MatchedComments int64     `json:"matched_comments"`
	ReportCount     int64     `json:"report_count"`
}

// KeywordRule 关键字/正则匹配规则
type KeywordRule struct {
	ID            uint       `json:"id" gorm:"primaryKey"`
	CreatedAt     time.Time  `json:"created_at"`
	UpdatedAt     time.Time  `json:"updated_at"`
	Name          string     `json:"name"`
	Pattern       string     `json:"pattern"`
	MatchType     string     `json:"match_type" gorm:"default:plain"`   // plain, regex
	MatchLogic    string     `json:"match_logic" gorm:"default:single"` // single, or, and
	CaseSensitive bool       `json:"case_sensitive"`
	Enabled       bool       `json:"enabled" gorm:"default:true"`
	Description   string     `json:"description"`
	LastMatchedAt *time.Time `json:"last_matched_at"`
}

// WhitelistUser 白名单用户，命中后跳过举报
type WhitelistUser struct {
	ID        uint      `json:"id" gorm:"primaryKey"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
	UID       int64     `json:"uid" gorm:"index"`
	Uname     string    `json:"uname" gorm:"index"`
	Remark    string    `json:"remark"`
	Enabled   bool      `json:"enabled" gorm:"default:true"`
}

// AppSetting 可视化配置项
type AppSetting struct {
	Key       string    `json:"key" gorm:"primaryKey"`
	Value     string    `json:"value"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MonitorLog 监控日志
type MonitorLog struct {
	ID          uint        `json:"id" gorm:"primaryKey"`
	CreatedAt   time.Time   `json:"created_at"`
	TaskID      uint        `json:"task_id"`
	Task        MonitorTask `json:"task" gorm:"foreignKey:TaskID"`
	Message     string      `json:"message"`
	Level       string      `json:"level"` // info, warning, error
	Digest      string      `json:"digest" gorm:"index"`
	RepeatCount int         `json:"repeat_count" gorm:"default:1"`
	LastSeenAt  *time.Time  `json:"last_seen_at"`
}

// ReportRecord 举报记录
type ReportRecord struct {
	ID              uint        `json:"id" gorm:"primaryKey"`
	CreatedAt       time.Time   `json:"created_at"`
	UpdatedAt       time.Time   `json:"updated_at"`
	TaskID          uint        `json:"task_id" gorm:"uniqueIndex:idx_task_comment"`
	Task            MonitorTask `json:"task" gorm:"foreignKey:TaskID"`
	TargetUID       int64       `json:"target_uid" gorm:"index"`
	TargetUname     string      `json:"target_uname"`
	AVID            int64       `json:"avid"`                                           // 视频AV号
	BVID            string      `json:"bvid"`                                           // 视频BV号
	VideoTitle      string      `json:"video_title"`                                    // 视频标题
	CommentID       int64       `json:"comment_id" gorm:"uniqueIndex:idx_task_comment"` // 评论ID
	CommentContent  string      `json:"comment_content"`                                // 评论内容
	CommentUser     string      `json:"comment_user"`                                   // 评论用户
	CommentUserID   int64       `json:"comment_user_id"`
	KeywordRuleID   *uint       `json:"keyword_rule_id"`
	KeywordRuleName string      `json:"keyword_rule_name"`
	MatchedKeyword  string      `json:"matched_keyword"` // 匹配的关键字
	MatchType       string      `json:"match_type"`
	Reason          int         `json:"reason" gorm:"default:11"` // 举报理由：11=传谣类
	Success         bool        `json:"success"`                  // 举报是否成功
	Message         string      `json:"message"`                  // 举报结果消息
}
