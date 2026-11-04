package messagestore

import "testing"

// 超时/失败路径关闭 ack 后必须从 map 移除，避免长期运行无界增长。
func TestAckChanRemovedAfterClose(t *testing.T) {
	store := &MessageStore{}
	ack := store.GenAckChan("msg-1")
	if _, ok := store.LoadMsgChan("msg-1"); !ok {
		t.Fatal("登记后应能查到 ack")
	}
	ack.TryCloseChan()
	if _, ok := store.LoadMsgChan("msg-1"); ok {
		t.Fatal("关闭后应从 map 移除")
	}
}

func TestAckChanRemovedAfterSend(t *testing.T) {
	store := &MessageStore{}
	ack := store.GenAckChan("msg-2")
	if !ack.TrySendDataAndCloseChan("resp") {
		t.Fatal("首次发送应成功")
	}
	if _, ok := store.LoadMsgChan("msg-2"); ok {
		t.Fatal("发送并关闭后应从 map 移除")
	}
	if ack.TrySendDataAndCloseChan("again") {
		t.Fatal("已关闭的 ack 不应再次发送")
	}
}
