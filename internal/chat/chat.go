// Package chat 定义 IM 提供方抽象。
//
// 设计目标：业务代码只依赖本包接口，不 import 任何具体 IM 厂商 SDK。
// 参考实现：外部自托管 IM（见 docs/decisions/0002-im-provider.md）、memory（本仓内置，测试用）。
package chat

import (
	"context"
	"errors"
	"time"
)

// ErrNotSupported 表示该提供方不支持此能力（例如在线状态）。
var ErrNotSupported = errors.New("chat: capability not supported")

// Session 是登录后的会话凭据。Token 由提供方签发，业务侧不解析其内容。
type Session struct {
	UserID    string    `json:"userId"`
	Token     string    `json:"token"`
	ExpiresAt time.Time `json:"expiresAt"`
}

// Message 是统一消息模型。ClientMsgID 用于幂等去重。
type Message struct {
	ConvID      string    `json:"convId"`
	MsgID       string    `json:"msgId"`
	ClientMsgID string    `json:"clientMsgId"`
	SenderID    string    `json:"senderId"`
	Type        string    `json:"type"` // text | image | custom
	Text        string    `json:"text,omitempty"`
	MediaURL    string    `json:"mediaUrl,omitempty"`
	Seq         int64     `json:"seq"`
	SentAt      time.Time `json:"sentAt"`
}

// Page 是历史消息分页结果。
type Page[T any] struct {
	Items      []T    `json:"items"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// ReadReceipt 是已读回执。
type ReadReceipt struct {
	ConvID string    `json:"convId"`
	UserID string    `json:"userId"`
	MsgID  string    `json:"msgId"`
	ReadAt time.Time `json:"readAt"`
}

// Group 是群组。
type Group struct {
	GroupID string   `json:"groupId"`
	Name    string   `json:"name"`
	Members []string `json:"members"`
}

// Provider 是 IM 提供方接口。实现方需保证：
//   - SendText 幂等：同一 ClientMsgID 重复调用不产生重复消息；
//   - GetHistory 按 Seq 单调，游标不丢消息；
//   - 回调/订阅断开时不得静默丢消息（至少返回错误）。
type Provider interface {
	// 身份：EnsureUser 保证 IM 侧存在该用户（幂等），Login 返回客户端连接凭据
	EnsureUser(ctx context.Context, userID, nickname, avatarURL string) error
	Login(ctx context.Context, userID, credential string) (Session, error)
	Logout(ctx context.Context, userID string) error

	// 消息
	SendText(ctx context.Context, convID, senderID, text, clientMsgID string) (Message, error)
	SendImage(ctx context.Context, convID, senderID, mediaURL, clientMsgID string) (Message, error)
	GetHistory(ctx context.Context, userID, convID, cursor string, limit int) (Page[Message], error)

	// 已读
	MarkRead(ctx context.Context, convID, userID, msgID string) error
	UnreadTotal(ctx context.Context, userID string) (int, error)

	// 群组
	CreateGroup(ctx context.Context, name string, members []string) (Group, error)
	GroupMembers(ctx context.Context, groupID string) ([]string, error)

	// 好友（服务端直接互加，无申请审批流程）
	AddFriend(ctx context.Context, userID string, friendIDs []string) error
	Friends(ctx context.Context, userID string) ([]string, error)

	// 订阅（服务端侧）
	Subscribe(ctx context.Context, convID string, fn func(Message)) (cancel func(), err error)

	// 可选能力，不支持时返回 ErrNotSupported
	Presence(ctx context.Context, userIDs []string) (map[string]bool, error)
}
