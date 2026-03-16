package core

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"
)

// -- mocks --

type mockBackend struct {
	mu       sync.Mutex
	lastReq  *BackendRequest
	resp     *Response
	err      error
	deltas   []string
	barrier  chan struct{} // if non-nil, Run blocks until closed
	called   bool
}

func (m *mockBackend) Run(ctx context.Context, req BackendRequest, deltaCh chan<- StreamDelta) (*Response, error) {
	m.mu.Lock()
	m.called = true
	reqCopy := req
	m.lastReq = &reqCopy
	m.mu.Unlock()

	if m.barrier != nil {
		select {
		case <-m.barrier:
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}
	for _, d := range m.deltas {
		deltaCh <- StreamDelta{Text: d}
	}
	return m.resp, m.err
}

type mockPlugin struct {
	resp   *Response
	err    error
	called bool
}

func (m *mockPlugin) Init(_ map[string]any) error { return nil }

func (m *mockPlugin) HandleMessage(_ context.Context, _ ChatMessage, deltaCh chan<- StreamDelta) (*Response, error) {
	m.called = true
	return m.resp, m.err
}

type mockStore struct {
	mu   sync.Mutex
	data map[string]string
	sets []string // keys that were Set
}

func newMockStore() *mockStore {
	return &mockStore{data: make(map[string]string)}
}

func (m *mockStore) Get(key string) string {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.data[key]
}

func (m *mockStore) Set(key, sessionID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.data[key] = sessionID
	m.sets = append(m.sets, key)
}

func (m *mockStore) Reset(key string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.data, key)
}

// -- helpers --

func testBridge(backend *mockBackend, store *mockStore, plugins ...Plugin) *Bridge {
	return NewBridge(
		map[string]FrontendRouting{
			"bot1": {BackendName: "default", Model: "sonnet", PermissionMode: "plan", SystemPrompt: "Be helpful."},
		},
		map[string]LLMBackend{"default": backend},
		plugins,
		map[string]SessionStore{"bot1": store},
	)
}

func testMsg(chatID, userID, text string) ChatMessage {
	return ChatMessage{ChatID: chatID, UserID: userID, Text: text, WorkingDir: "/tmp"}
}

func callHandler(t *testing.T, b *Bridge, frontend string, msg ChatMessage) (*Response, error) {
	t.Helper()
	h := b.Handler(frontend)
	deltaCh := make(chan StreamDelta, 100)
	go func() {
		for range deltaCh {
		}
	}()
	return h(context.Background(), msg, deltaCh)
}

// -- tests --

func TestHandlerRoutesToBackend(t *testing.T) {
	b1 := &mockBackend{resp: &Response{SessionID: "s1"}}
	b2 := &mockBackend{resp: &Response{SessionID: "s2"}}
	bridge := NewBridge(
		map[string]FrontendRouting{
			"fe1": {BackendName: "b1"},
			"fe2": {BackendName: "b2"},
		},
		map[string]LLMBackend{"b1": b1, "b2": b2},
		nil,
		map[string]SessionStore{"fe1": newMockStore(), "fe2": newMockStore()},
	)

	callHandler(t, bridge, "fe1", testMsg("1", "1", "hello"))
	callHandler(t, bridge, "fe2", testMsg("2", "2", "world"))

	if !b1.called {
		t.Error("b1 should be called for fe1")
	}
	if !b2.called {
		t.Error("b2 should be called for fe2")
	}
}

func TestBackendRequestFromRouting(t *testing.T) {
	agent := &AgentDef{Name: "coder", Description: "Code", Prompt: "Write code.", Tools: []string{"Read"}}
	backend := &mockBackend{resp: &Response{SessionID: "s1"}}
	store := newMockStore()
	store.Set("100:200", "prev-session")

	bridge := NewBridge(
		map[string]FrontendRouting{
			"bot1": {BackendName: "default", Model: "opus", PermissionMode: "bypass", SystemPrompt: "sys", Agent: agent},
		},
		map[string]LLMBackend{"default": backend},
		nil,
		map[string]SessionStore{"bot1": store},
	)

	msg := ChatMessage{ChatID: "100", UserID: "200", Text: "prompt", WorkingDir: "/work"}
	callHandler(t, bridge, "bot1", msg)

	req := backend.lastReq
	if req.Prompt != "prompt" {
		t.Errorf("Prompt = %q, want %q", req.Prompt, "prompt")
	}
	if req.WorkingDir != "/work" {
		t.Errorf("WorkingDir = %q, want %q", req.WorkingDir, "/work")
	}
	if req.Model != "opus" {
		t.Errorf("Model = %q, want %q", req.Model, "opus")
	}
	if req.PermissionMode != "bypass" {
		t.Errorf("PermissionMode = %q, want %q", req.PermissionMode, "bypass")
	}
	if req.SystemPrompt != "sys" {
		t.Errorf("SystemPrompt = %q, want %q", req.SystemPrompt, "sys")
	}
	if req.SessionID != "prev-session" {
		t.Errorf("SessionID = %q, want %q", req.SessionID, "prev-session")
	}
	if req.Agent == nil || req.Agent.Name != "coder" {
		t.Errorf("Agent = %v, want coder", req.Agent)
	}
}

func TestPluginShortCircuit(t *testing.T) {
	p1 := &mockPlugin{resp: &Response{Text: "plugin response"}}
	p2 := &mockPlugin{}
	backend := &mockBackend{resp: &Response{}}

	bridge := testBridge(backend, newMockStore(), p1, p2)
	resp, err := callHandler(t, bridge, "bot1", testMsg("1", "1", "hi"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Text != "plugin response" {
		t.Errorf("Text = %q, want %q", resp.Text, "plugin response")
	}
	if p2.called {
		t.Error("p2 should not be called")
	}
	if backend.called {
		t.Error("backend should not be called")
	}
}

func TestPluginPassThrough(t *testing.T) {
	p1 := &mockPlugin{resp: nil}
	p2 := &mockPlugin{resp: nil}
	backend := &mockBackend{resp: &Response{Text: "backend response"}}

	bridge := testBridge(backend, newMockStore(), p1, p2)
	resp, err := callHandler(t, bridge, "bot1", testMsg("1", "1", "hi"))

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !p1.called || !p2.called {
		t.Error("both plugins should be called")
	}
	if !backend.called {
		t.Error("backend should be called")
	}
	if resp.Text != "backend response" {
		t.Errorf("Text = %q, want %q", resp.Text, "backend response")
	}
}

func TestConcurrencyErrRequestActive(t *testing.T) {
	barrier := make(chan struct{})
	backend := &mockBackend{resp: &Response{SessionID: "s1"}, barrier: barrier}
	bridge := testBridge(backend, newMockStore())

	msg := testMsg("1", "1", "hi")

	// First request blocks
	var firstResp *Response
	var firstErr error
	done := make(chan struct{})
	go func() {
		firstResp, firstErr = callHandler(t, bridge, "bot1", msg)
		close(done)
	}()

	// Wait for backend to be called
	for i := 0; i < 100; i++ {
		backend.mu.Lock()
		called := backend.called
		backend.mu.Unlock()
		if called {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Second request should get ErrRequestActive
	_, secondErr := callHandler(t, bridge, "bot1", msg)
	if !errors.Is(secondErr, ErrRequestActive) {
		t.Errorf("second request error = %v, want ErrRequestActive", secondErr)
	}

	close(barrier)
	<-done

	if firstErr != nil {
		t.Errorf("first request error = %v, want nil", firstErr)
	}
	if firstResp == nil {
		t.Error("first request should have response")
	}
}

func TestCrossChatIsolation(t *testing.T) {
	barrier := make(chan struct{})
	backend := &mockBackend{resp: &Response{}, barrier: barrier}
	bridge := testBridge(backend, newMockStore())

	done1 := make(chan struct{})
	go func() {
		callHandler(t, bridge, "bot1", testMsg("chat1", "user1", "hi"))
		close(done1)
	}()

	// Wait for first to be active
	for i := 0; i < 100; i++ {
		if bridge.HasActive("bot1", "chat1", "user1") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Same user, different chat should succeed
	done2 := make(chan struct{})
	go func() {
		callHandler(t, bridge, "bot1", testMsg("chat2", "user1", "hi"))
		close(done2)
	}()

	close(barrier)
	<-done1
	<-done2
}

func TestCancelRequest(t *testing.T) {
	barrier := make(chan struct{})
	backend := &mockBackend{resp: &Response{}, barrier: barrier}
	bridge := testBridge(backend, newMockStore())

	done := make(chan struct{})
	go func() {
		callHandler(t, bridge, "bot1", testMsg("1", "1", "hi"))
		close(done)
	}()

	for i := 0; i < 100; i++ {
		if bridge.HasActive("bot1", "1", "1") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if !bridge.CancelRequest("bot1", "1", "1") {
		t.Error("CancelRequest should return true")
	}

	close(barrier)
	<-done

	if bridge.CancelRequest("bot1", "1", "1") {
		t.Error("second CancelRequest should return false")
	}
}

func TestCancelRequestKeepsActiveUntilUnwind(t *testing.T) {
	barrier := make(chan struct{})
	backend := &mockBackend{resp: &Response{SessionID: "s1"}, barrier: barrier}
	bridge := testBridge(backend, newMockStore())
	msg := testMsg("1", "1", "hi")

	done1 := make(chan struct{})
	go func() {
		callHandler(t, bridge, "bot1", msg)
		close(done1)
	}()

	for i := 0; i < 100; i++ {
		if bridge.HasActive("bot1", "1", "1") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	// Cancel but do NOT unblock barrier
	bridge.CancelRequest("bot1", "1", "1")

	// Key: HasActive should still be true (CancelRequest does not delete from map)
	if !bridge.HasActive("bot1", "1", "1") {
		t.Error("HasActive should remain true until handler unwinds")
	}

	// Unblock barrier so first handler can complete
	close(barrier)
	<-done1

	// Now active should be cleared
	if bridge.HasActive("bot1", "1", "1") {
		t.Error("HasActive should be false after unwind")
	}
}

func TestResetSession(t *testing.T) {
	store := newMockStore()
	store.Set("100:200", "sess-old")
	bridge := testBridge(&mockBackend{resp: &Response{}}, store)

	bridge.ResetSession("bot1", "100", "200")

	if got := store.Get("100:200"); got != "" {
		t.Errorf("session after reset = %q, want empty", got)
	}
}

func TestSessionIDFromStore(t *testing.T) {
	s1 := newMockStore()
	s1.Set("1:2", "sess-a")
	s2 := newMockStore()
	s2.Set("1:2", "sess-b")

	bridge := NewBridge(
		map[string]FrontendRouting{"fe1": {BackendName: "default"}, "fe2": {BackendName: "default"}},
		map[string]LLMBackend{"default": &mockBackend{resp: &Response{}}},
		nil,
		map[string]SessionStore{"fe1": s1, "fe2": s2},
	)

	if got := bridge.SessionID("fe1", "1", "2"); got != "sess-a" {
		t.Errorf("fe1 session = %q, want %q", got, "sess-a")
	}
	if got := bridge.SessionID("fe2", "1", "2"); got != "sess-b" {
		t.Errorf("fe2 session = %q, want %q", got, "sess-b")
	}
}

func TestDeltaChClosedByHandler(t *testing.T) {
	backend := &mockBackend{resp: &Response{}, deltas: []string{"a", "b"}}
	bridge := testBridge(backend, newMockStore())

	h := bridge.Handler("bot1")
	deltaCh := make(chan StreamDelta, 100)

	var collected []string
	done := make(chan struct{})
	go func() {
		for d := range deltaCh {
			collected = append(collected, d.Text)
		}
		close(done)
	}()

	h(context.Background(), testMsg("1", "1", "hi"), deltaCh)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("deltaCh was not closed")
	}

	if len(collected) != 2 {
		t.Errorf("collected %d deltas, want 2", len(collected))
	}
}

func TestDeltaChClosedAfterPlugin(t *testing.T) {
	p := &mockPlugin{resp: &Response{Text: "plugin"}}
	bridge := testBridge(&mockBackend{}, newMockStore(), p)

	h := bridge.Handler("bot1")
	deltaCh := make(chan StreamDelta, 100)

	done := make(chan struct{})
	go func() {
		for range deltaCh {
		}
		close(done)
	}()

	h(context.Background(), testMsg("1", "1", "hi"), deltaCh)

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("deltaCh was not closed after plugin handled message")
	}
}

func TestHasActive(t *testing.T) {
	barrier := make(chan struct{})
	backend := &mockBackend{resp: &Response{}, barrier: barrier}
	bridge := testBridge(backend, newMockStore())

	if bridge.HasActive("bot1", "1", "1") {
		t.Error("should not be active before request")
	}

	done := make(chan struct{})
	go func() {
		callHandler(t, bridge, "bot1", testMsg("1", "1", "hi"))
		close(done)
	}()

	for i := 0; i < 100; i++ {
		if bridge.HasActive("bot1", "1", "1") {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}

	if !bridge.HasActive("bot1", "1", "1") {
		t.Error("should be active during request")
	}

	close(barrier)
	<-done

	if bridge.HasActive("bot1", "1", "1") {
		t.Error("should not be active after request")
	}
}

func TestSessionSavedFromResponse(t *testing.T) {
	backend := &mockBackend{resp: &Response{SessionID: "new-sess"}}
	store := newMockStore()
	bridge := testBridge(backend, store)

	callHandler(t, bridge, "bot1", testMsg("10", "20", "hi"))

	if got := store.Get("10:20"); got != "new-sess" {
		t.Errorf("session = %q, want %q", got, "new-sess")
	}
}

func TestSessionNotSavedOnEmpty(t *testing.T) {
	backend := &mockBackend{resp: &Response{SessionID: ""}}
	store := newMockStore()
	bridge := testBridge(backend, store)

	callHandler(t, bridge, "bot1", testMsg("10", "20", "hi"))

	if len(store.sets) != 0 {
		t.Errorf("Set called %d times, want 0 for empty SessionID", len(store.sets))
	}
}

func TestPerFrontendSessionIsolation(t *testing.T) {
	b := &mockBackend{resp: &Response{SessionID: "shared-sess"}}
	s1 := newMockStore()
	s2 := newMockStore()
	bridge := NewBridge(
		map[string]FrontendRouting{
			"fe1": {BackendName: "default"},
			"fe2": {BackendName: "default"},
		},
		map[string]LLMBackend{"default": b},
		nil,
		map[string]SessionStore{"fe1": s1, "fe2": s2},
	)

	callHandler(t, bridge, "fe1", testMsg("1", "1", "hi"))
	callHandler(t, bridge, "fe2", testMsg("1", "1", "hi"))

	if s1.Get("1:1") != "shared-sess" {
		t.Error("fe1 store should have session")
	}
	if s2.Get("1:1") != "shared-sess" {
		t.Error("fe2 store should have session")
	}
}
