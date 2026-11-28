// Package dataset 定义数据集抽象（公开素材镜像 / 冷备）。
//
// 重要约束（来自阶段 0 审计，2026-11-27）：
//   - 数据集提供方【不得】用于用户上传文件主存储：
//     写模型是 commit+LFS、私有=整仓、无对象级 ACL、删除不可编程、commit 频率限流不公开；
//   - 仅用于：公开素材镜像（表情包/贴纸/公开数据包）、冷备归档；
//   - 用户文件一律走 storage.Provider。
package dataset

import (
	"context"
	"time"
)

// Repo 是数据集仓库。
type Repo struct {
	Namespace  string `json:"namespace"`
	Name       string `json:"name"`
	Visibility string `json:"visibility"` // public | private
}

// FileEntry 是仓库内文件。
type FileEntry struct {
	Path        string    `json:"path"`
	Size        int64     `json:"size"`
	SHA256      string    `json:"sha256"`
	CommittedAt time.Time `json:"committedAt"`
}

// Provider 是数据集提供方接口，只实现镜像/冷备所需能力。
type Provider interface {
	// 只读
	List(ctx context.Context, repo Repo, revision, prefix string) ([]FileEntry, error)
	DownloadURL(ctx context.Context, repo Repo, revision, path string) (string, error)

	// 批量写：实现方应把一次调用合并成一次 commit（避免触发限流）
	UploadBatch(ctx context.Context, repo Repo, files map[string][]byte, message string) (commitID string, err error)
	DeleteBatch(ctx context.Context, repo Repo, paths []string, message string) (commitID string, err error)
}
