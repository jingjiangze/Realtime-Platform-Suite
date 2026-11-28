// Package memory 是 chat.Provider 的内存实现，仅用于测试与本地开发。
package memory

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/jingjiangze/realtime-platform-foundation/internal/chat"
)

// Provider 是进程内 IM 实现，重启即丢数据。
type Provider struct {
	mu       sync.RWMutex
	msgs     map[string][]chat.Message // convID -> 按 Seq 递增
	seen     map[string]struct{}       // clientMsgID 去重
	groups   map[string]chat.Group
	friends  map[string]struct{}                      // userID|friendID
	readMark map[string]string                        // convID|userID -> msgID
	subs     map[string]map[uint64]func(chat.Message) // convID -> subID -> 回调
	nextSub  uint64
	seq      map[string]int64 // convID -> 最后 Seq
}

// New 创建内存提供方。
func New() *Provider {
	return &Provider{
		msgs:     make(map[string][]chat.Message),
		seen:     make(map[string]struct{}),
		groups:   make(map[string]chat.Group),
		friends:  make(map[string]struct{}),
		readMark: make(map[string]string),
		subs:     make(map[string]map[uint64]func(chat.Message)),
		seq:      make(map[string]int64),
	}
}

// newID 生成随机 ID（避免引入外部依赖）。
func newID() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// EnsureUser 内存实现总是成功（用户按需出现）。
func (p *Provider) EnsureUser(context.Context, string, string, string) error { return nil }

// Login 直接签发一个假 token。
func (p *Provider) Login(_ context.Context, userID, _ string) (chat.Session, error) {
	if userID == "" {
		return chat.Session{}, fmt.Errorf("memory: userID required")
	}
	return chat.Session{UserID: userID, Token: "mem-" + userID, ExpiresAt: time.Now().Add(time.Hour)}, nil
}

// Logout 无状态，直接成功。
func (p *Provider) Logout(context.Context, string) error { return nil }

// SendText 幂等写入一条文本消息。
func (p *Provider) SendText(_ context.Context, convID, senderID, text, clientMsgID string) (chat.Message, error) {
	return p.append(convID, senderID, "text", text, "", clientMsgID)
}

// SendImage 幂等写入一条图片消息。
func (p *Provider) SendImage(_ context.Context, convID, senderID, mediaURL, clientMsgID string) (chat.Message, error) {
	return p.append(convID, senderID, "image", "", mediaURL, clientMsgID)
}

func (p *Provider) append(convID, senderID, typ, text, mediaURL, clientMsgID string) (chat.Message, error) {
	if convID == "" || senderID == "" || clientMsgID == "" {
		return chat.Message{}, fmt.Errorf("memory: convID/senderID/clientMsgID required")
	}

	p.mu.Lock()
	defer p.mu.Unlock()

	// 幂等：同一 clientMsgID 直接返回已有消息
	if _, ok := p.seen[clientMsgID]; ok {
		for _, m := range p.msgs[convID] {
			if m.ClientMsgID == clientMsgID {
				return m, nil
			}
		}
	}

	p.seq[convID]++
	msg := chat.Message{
		ConvID:      convID,
		MsgID:       newID(),
		ClientMsgID: clientMsgID,
		SenderID:    senderID,
		Type:        typ,
		Text:        text,
		MediaURL:    mediaURL,
		Seq:         p.seq[convID],
		SentAt:      time.Now().UTC(),
	}
	p.msgs[convID] = append(p.msgs[convID], msg)
	p.seen[clientMsgID] = struct{}{}

	for _, fn := range p.subs[convID] {
		go fn(msg)
	}

	return msg, nil
}

// GetHistory 按 Seq 升序分页；cursor 为上一页最后一条的 Seq（十进制）。
func (p *Provider) GetHistory(_ context.Context, _ string, convID, cursor string, limit int) (chat.Page[chat.Message], error) {
	if limit <= 0 || limit > 200 {
		limit = 50
	}
	var after int64
	if cursor != "" {
		if _, err := fmt.Sscanf(cursor, "%d", &after); err != nil {
			return chat.Page[chat.Message]{}, fmt.Errorf("memory: bad cursor %q", cursor)
		}
	}

	p.mu.RLock()
	defer p.mu.RUnlock()

	all := p.msgs[convID]
	idx := sort.Search(len(all), func(i int) bool { return all[i].Seq > after })
	out := make([]chat.Message, 0, limit)
	for i := idx; i < len(all) && len(out) < limit; i++ {
		out = append(out, all[i])
	}
	page := chat.Page[chat.Message]{Items: out}
	if len(out) == limit && idx+limit < len(all) {
		page.NextCursor = fmt.Sprint(out[len(out)-1].Seq)
	}
	return page, nil
}

// MarkRead 记录已读位置。
func (p *Provider) MarkRead(_ context.Context, convID, userID, msgID string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.readMark[convID+"|"+userID] = msgID
	return nil
}

// UnreadTotal 统计未读（简化：总消息数减去已读位置之前的部分）。
func (p *Provider) UnreadTotal(_ context.Context, userID string) (int, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	total := 0
	for convID, msgs := range p.msgs {
		mark, ok := p.readMark[convID+"|"+userID]
		if !ok {
			total += len(msgs)
			continue
		}
		for _, m := range msgs {
			if m.MsgID == mark {
				break
			}
			total++
		}
	}
	return total, nil
}

// CreateGroup 建群。
func (p *Provider) CreateGroup(_ context.Context, name string, members []string) (chat.Group, error) {
	p.mu.Lock()
	defer p.mu.Unlock()
	g := chat.Group{GroupID: newID(), Name: name, Members: append([]string(nil), members...)}
	p.groups[g.GroupID] = g
	return g, nil
}

// GroupMembers 返回群成员。
func (p *Provider) GroupMembers(_ context.Context, groupID string) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	g, ok := p.groups[groupID]
	if !ok {
		return nil, fmt.Errorf("memory: group %s not found", groupID)
	}
	return append([]string(nil), g.Members...), nil
}

// AddFriend 建立好友关系（内存实现双向都建，贴近 IM 语义）。
func (p *Provider) AddFriend(_ context.Context, userID string, friendIDs []string) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for _, fid := range friendIDs {
		p.friends[userID+"|"+fid] = struct{}{}
		p.friends[fid+"|"+userID] = struct{}{}
	}
	return nil
}

// Friends 返回好友 ID 列表。
func (p *Provider) Friends(_ context.Context, userID string) ([]string, error) {
	p.mu.RLock()
	defer p.mu.RUnlock()
	ids := make([]string, 0, len(p.friends))
	prefix := userID + "|"
	for k := range p.friends {
		if strings.HasPrefix(k, prefix) {
			ids = append(ids, strings.TrimPrefix(k, prefix))
		}
	}
	sort.Strings(ids)
	return ids, nil
}

// Subscribe 注册消息订阅，返回取消函数。
func (p *Provider) Subscribe(_ context.Context, convID string, fn func(chat.Message)) (func(), error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	p.nextSub++
	id := p.nextSub
	if p.subs[convID] == nil {
		p.subs[convID] = make(map[uint64]func(chat.Message))
	}
	p.subs[convID][id] = fn

	var once sync.Once
	return func() {
		once.Do(func() {
			p.mu.Lock()
			defer p.mu.Unlock()
			delete(p.subs[convID], id)
		})
	}, nil
}

// Presence 内存实现不支持在线状态。
func (p *Provider) Presence(context.Context, []string) (map[string]bool, error) {
	return nil, chat.ErrNotSupported
}
