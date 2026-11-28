package memory_test

import (
	"context"
	"testing"
	"time"

	"github.com/jingjiangze/realtime-platform-foundation/internal/chat"
	"github.com/jingjiangze/realtime-platform-foundation/internal/chat/memory"
)

// timeAfter 返回一个短超时通道，用于断言「不应发生」的事件。
func timeAfter() <-chan time.Time { return time.After(200 * time.Millisecond) }

func TestSendText_Idempotent(t *testing.T) {
	p := memory.New()
	ctx := context.Background()

	first, err := p.SendText(ctx, "c1", "u1", "hello", "cid-1")
	if err != nil {
		t.Fatalf("first send: %v", err)
	}

	again, err := p.SendText(ctx, "c1", "u1", "hello", "cid-1")
	if err != nil {
		t.Fatalf("second send: %v", err)
	}

	if first.MsgID != again.MsgID {
		t.Fatalf("同一 clientMsgID 必须返回同一条消息: %s != %s", first.MsgID, again.MsgID)
	}

	page, err := p.GetHistory(ctx, "u1", "c1", "", 10)
	if err != nil {
		t.Fatalf("history: %v", err)
	}
	if len(page.Items) != 1 {
		t.Fatalf("重复发送不得产生第二条消息，实际 %d 条", len(page.Items))
	}
}

func TestGetHistory_SeqMonotonic(t *testing.T) {
	p := memory.New()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		if _, err := p.SendText(ctx, "c1", "u1", "m", string(rune('a'+i))); err != nil {
			t.Fatalf("send %d: %v", i, err)
		}
	}

	page, err := p.GetHistory(ctx, "u1", "c1", "", 3)
	if err != nil {
		t.Fatalf("page1: %v", err)
	}
	if len(page.Items) != 3 {
		t.Fatalf("期望 3 条，实际 %d", len(page.Items))
	}
	if page.NextCursor == "" {
		t.Fatal("期望有 NextCursor")
	}
	for i := 1; i < len(page.Items); i++ {
		if page.Items[i].Seq <= page.Items[i-1].Seq {
			t.Fatalf("Seq 必须单调递增: %d <= %d", page.Items[i].Seq, page.Items[i-1].Seq)
		}
	}

	next, err := p.GetHistory(ctx, "u1", "c1", page.NextCursor, 3)
	if err != nil {
		t.Fatalf("page2: %v", err)
	}
	if len(next.Items) != 2 {
		t.Fatalf("游标续页不得丢消息，期望 2 条，实际 %d", len(next.Items))
	}
	if next.Items[0].Seq <= page.Items[2].Seq {
		t.Fatal("续页首条 Seq 必须大于上一页末条")
	}
}

func TestFriends_Bidirectional(t *testing.T) {
	p := memory.New()
	ctx := context.Background()

	if err := p.AddFriend(ctx, "u1", []string{"u2", "u3"}); err != nil {
		t.Fatalf("add friend: %v", err)
	}
	got, err := p.Friends(ctx, "u2")
	if err != nil {
		t.Fatalf("friends: %v", err)
	}
	if len(got) != 1 || got[0] != "u1" {
		t.Fatalf("双向好友期望 [u1]，实际 %v", got)
	}
}

func TestCreateGroup_AndMembers(t *testing.T) {
	p := memory.New()
	ctx := context.Background()

	g, err := p.CreateGroup(ctx, "球友局", []string{"u1", "u2"})
	if err != nil {
		t.Fatalf("create group: %v", err)
	}
	ids, err := p.GroupMembers(ctx, g.GroupID)
	if err != nil {
		t.Fatalf("group members: %v", err)
	}
	if len(ids) != 2 {
		t.Fatalf("期望 2 名成员，实际 %v", ids)
	}
}

func TestPresence_NotSupported(t *testing.T) {
	p := memory.New()
	if _, err := p.Presence(context.Background(), []string{"u1"}); err != chat.ErrNotSupported {
		t.Fatalf("期望 ErrNotSupported，实际 %v", err)
	}
}

func TestSubscribe_ReceivesAndCancels(t *testing.T) {
	p := memory.New()
	ctx := context.Background()

	got := make(chan chat.Message, 1)
	cancel, err := p.Subscribe(ctx, "c1", func(m chat.Message) { got <- m })
	if err != nil {
		t.Fatalf("subscribe: %v", err)
	}

	if _, err := p.SendText(ctx, "c1", "u1", "hi", "cid-1"); err != nil {
		t.Fatalf("send: %v", err)
	}
	select {
	case m := <-got:
		if m.Text != "hi" {
			t.Fatalf("消息内容不符: %q", m.Text)
		}
	case <-timeAfter():
		t.Fatal("订阅者未收到消息")
	}

	cancel()
	if _, err := p.SendText(ctx, "c1", "u1", "after-cancel", "cid-2"); err != nil {
		t.Fatalf("send2: %v", err)
	}
	select {
	case <-got:
		t.Fatal("取消后不应再收到消息")
	case <-timeAfter():
	}
}
