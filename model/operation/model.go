package operation

import (
	"github.com/open4go/model"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

const (
	// CollectionNamePrefix 数据库表前缀
	// 可以根据具体业务的需要进行定义
	// 例如: sys_, scm_, customer_, order_ 等
	collectionNamePrefix = "auth_"
	// CollectionNameSuffix 后缀
	// 例如, _log, _config, _flow,
	collectionNameSuffix = "_log"
	// 这个需要用户根据具体业务完成设定
	modelName = "operation"
)

// 每一个应用表示一个大的模块，通常其子模块是一个个接口
// 是有系统默认设定，用户无需修改
// 用户只需要在创建角色的时候选择好需要的应用即可
// 用户选择所需要的应用后->完成角色创建->系统自动拷贝应用具体信息到角色下
// 此时用户可以针对当前的角色中具体的项再自行选择是否移除部分接口，从而进行更精细的权限管理

// Model 模型
type Model struct {
	// 继承
	model.Model
	// 基本的数据库模型字段，一般情况所有model都应该包含如下字段
	// 创建时（用户上传的数据为空，所以默认可以不传该值)
	ID primitive.ObjectID `json:"id" bson:"_id,omitempty"`

	Timestamp uint64 `json:"timestamp" bson:"timestamp"`
	// 用户根据业务需求定义的字段
	// 客户IP
	ClientIP string `json:"client_ip" bson:"client_ip"`
	// 远程IP
	RemoteIP string `json:"remote_ip"  bson:"remote_ip"`
	// 路径
	FullPath string `json:"full_path"  bson:"full_path"`
	// 请求方法/操作
	Method string `json:"method"  bson:"method"`
	// 相应代码
	RespCode int `json:"resp_code"  bson:"resp_code"`
	// 操作对象id
	TargetID string `json:"target_id"  bson:"target_id"`
	// 设备号
	Device string `json:"device"  bson:"device"`
	// 操作人
	Operator string `json:"operator"  bson:"operator"`
	// 用户id
	UserID string `json:"user_id"  bson:"user_id"`
	// 账号id
	AccountID string `json:"account_id"  bson:"account_id"`
	// 修改前
	Before string `json:"before"  bson:"before"`
	// 修改后
	After string `json:"after"  bson:"after"`
	// Summary 可读摘要，例如「张三 更新了 会员 李四」
	Summary string `json:"summary" bson:"summary"`
	// Resource 资源编码，例如 member / admin_account / role。用量统计仍用这一层。
	Resource string `json:"resource" bson:"resource"`
	// Service x9 服务编码，例如 client、member、order、auth。展示时按语言翻译，不写死中文。
	Service string `json:"service" bson:"service"`
	// Module 服务下的具体模块，例如 launch、account、commands。
	Module string `json:"module" bson:"module"`
	// ResourceLabel 默认中文名，例如「终端管理 · 启动配置」（避免与 ResourceName() 方法冲突）
	ResourceLabel string `json:"resource_label" bson:"resource_label"`
	// Action 业务动作：create / update / delete / notify
	Action string `json:"action" bson:"action"`
	// RequestURI 实际请求路径（已脱敏）
	RequestURI string `json:"request_uri" bson:"request_uri"`
	// UserAgent 浏览器 / 客户端
	UserAgent string `json:"user_agent" bson:"user_agent"`
	// MerchantID 操作发生的租户
	MerchantID string `json:"merchant_id" bson:"merchant_id"`
	// LatencyMs 接口耗时
	LatencyMs int64 `json:"latency_ms" bson:"latency_ms"`
	// TraceID 请求追踪 id，与应用日志关联
	TraceID string `json:"trace_id" bson:"trace_id"`
	// TargetName 被操作对象的可读名称（会员名、账号名、手机号等）
	TargetName string `json:"target_name" bson:"target_name"`
	// OperatorPhone 操作人手机号
	OperatorPhone string `json:"operator_phone" bson:"operator_phone"`
	// LoginType 操作人登录类型（admin / member 等）
	LoginType string `json:"login_type" bson:"login_type"`
}
