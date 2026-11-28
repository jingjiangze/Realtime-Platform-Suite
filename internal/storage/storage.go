// Package storage 定义对象存储抽象（用户文件主存储）。
//
// 设计约束（来自阶段 0 审计）：
//   - 客户端永不接触云端凭据，只拿短时签名 URL；
//   - 私有对象必须走签名，公开对象可走 CDN；
//   - 大文件必须支持分片 + 续传状态外置（BOS Go SDK 无内置 checkpoint）。
package storage

import (
	"context"
	"errors"
	"time"
)

// ErrNotFound 表示对象不存在。
var ErrNotFound = errors.New("storage: object not found")

// ObjectInfo 是对象元数据。
type ObjectInfo struct {
	Key         string    `json:"key"`
	Size        int64     `json:"size"`
	ContentType string    `json:"contentType"`
	SHA256      string    `json:"sha256"`
	ModifiedAt  time.Time `json:"modifiedAt"`
}

// PresignOptions 控制签名 URL 的生成。
type PresignOptions struct {
	Method      string        // GET | PUT
	ExpiresIn   time.Duration // 上限由实现方决定
	ContentType string
}

// MultipartSession 表示一次分片上传的会话，可持久化以支持续传。
type MultipartSession struct {
	UploadID string            `json:"uploadId"`
	Key      string            `json:"key"`
	Parts    map[int]PartState `json:"parts"`
}

// PartState 记录单个分片的上传状态。
type PartState struct {
	ETag string `json:"etag"`
	Size int64  `json:"size"`
}

// Provider 是对象存储提供方接口。
type Provider interface {
	// 元数据
	Stat(ctx context.Context, key string) (ObjectInfo, error)
	Delete(ctx context.Context, key string) error

	// 直传：客户端用签名 URL 上传，服务端只签名
	PresignPut(ctx context.Context, key string, opts PresignOptions) (string, error)
	PresignGet(ctx context.Context, key string, opts PresignOptions) (string, error)

	// 服务端侧上传（小文件或回调场景）
	Put(ctx context.Context, key string, data []byte, contentType string) (ObjectInfo, error)
	Get(ctx context.Context, key string) ([]byte, error)

	// 分片上传 + 续传
	InitMultipart(ctx context.Context, key string, contentType string) (MultipartSession, error)
	UploadPart(ctx context.Context, sess MultipartSession, partNum int, data []byte) (PartState, error)
	CompleteMultipart(ctx context.Context, sess MultipartSession) (ObjectInfo, error)
	AbortMultipart(ctx context.Context, sess MultipartSession) error
}
