package core

import (
	"errors"
	"testing"
)

func TestAttachmentTypeConstants(t *testing.T) {
	tests := []struct {
		name     string
		got      AttachmentType
		expected string
	}{
		{"image", AttachmentImage, "image"},
		{"audio", AttachmentAudio, "audio"},
		{"video", AttachmentVideo, "video"},
		{"document", AttachmentDocument, "document"},
		{"voice", AttachmentVoice, "voice"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if string(tc.got) != tc.expected {
				t.Errorf("AttachmentType %s = %q, want %q", tc.name, tc.got, tc.expected)
			}
		})
	}
}

func TestChatMessageConstruction(t *testing.T) {
	tests := []struct {
		name string
		msg  ChatMessage
	}{
		{
			name: "text only",
			msg: ChatMessage{
				ID:     "1",
				ChatID: "100",
				UserID: "100",
				Text:   "hello",
			},
		},
		{
			name: "with attachments and working dir",
			msg: ChatMessage{
				ID:         "2",
				ChatID:     "200",
				UserID:     "201",
				Text:       "[Photo saved to /tmp/photo.jpg]\nDescribe this",
				WorkingDir: "/home/user/project",
				Attachments: []Attachment{
					{Type: AttachmentImage, Path: "/tmp/photo.jpg", Name: "photo.jpg", Size: 1024},
					{Type: AttachmentVoice, Path: "/tmp/voice.ogg", Name: "voice.ogg", Duration: 5},
				},
			},
		},
		{
			name: "command message",
			msg: ChatMessage{
				ID:        "3",
				ChatID:    "300",
				UserID:    "300",
				IsCommand: true,
				Command:   "new",
			},
		},
		{
			name: "with metadata",
			msg: ChatMessage{
				ID:       "4",
				ChatID:   "400",
				UserID:   "400",
				Text:     "test",
				Metadata: map[string]string{"debug": "true"},
			},
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.msg.ID == "" {
				t.Error("ID should not be empty")
			}
			if tc.msg.ChatID == "" {
				t.Error("ChatID should not be empty")
			}
			if tc.msg.UserID == "" {
				t.Error("UserID should not be empty")
			}
		})
	}
}

func TestResponseWithError(t *testing.T) {
	tests := []struct {
		name    string
		resp    Response
		wantErr bool
	}{
		{
			name: "success",
			resp: Response{
				Text:      "Hello, world!",
				SessionID: "sess-123",
				CostUSD:   0.05,
				NumTurns:  3,
			},
			wantErr: false,
		},
		{
			name: "error response",
			resp: Response{
				IsError: true,
				Error:   "something went wrong",
			},
			wantErr: true,
		},
		{
			name: "with cost and turns",
			resp: Response{
				SessionID: "sess-456",
				CostUSD:   1.23,
				NumTurns:  10,
			},
			wantErr: false,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.resp.IsError != tc.wantErr {
				t.Errorf("IsError = %v, want %v", tc.resp.IsError, tc.wantErr)
			}
			if tc.wantErr && tc.resp.Error == "" {
				t.Error("error response should have non-empty Error field")
			}
		})
	}
}

func TestBackendRequestWithAgentDef(t *testing.T) {
	tests := []struct {
		name      string
		req       BackendRequest
		wantAgent bool
		wantTools int // -1 = nil tools
	}{
		{
			name:      "no agent",
			req:       BackendRequest{Prompt: "hello", Model: "sonnet"},
			wantAgent: false,
		},
		{
			name: "agent with tools",
			req: BackendRequest{
				Prompt: "hello",
				Agent: &AgentDef{
					Name:        "coder",
					Description: "Code assistant",
					Prompt:      "You write code.",
					Tools:       []string{"Read", "Write", "Bash"},
				},
			},
			wantAgent: true,
			wantTools: 3,
		},
		{
			name: "agent with empty tools (disables all)",
			req: BackendRequest{
				Prompt: "hello",
				Agent:  &AgentDef{Name: "restricted", Tools: []string{}},
			},
			wantAgent: true,
			wantTools: 0,
		},
		{
			name: "agent with nil tools (defaults)",
			req: BackendRequest{
				Prompt: "hello",
				Agent:  &AgentDef{Name: "default"},
			},
			wantAgent: true,
			wantTools: -1,
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			hasAgent := tc.req.Agent != nil
			if hasAgent != tc.wantAgent {
				t.Errorf("Agent present = %v, want %v", hasAgent, tc.wantAgent)
			}
			if !tc.wantAgent {
				return
			}
			if tc.wantTools == -1 {
				if tc.req.Agent.Tools != nil {
					t.Errorf("Tools = %v, want nil", tc.req.Agent.Tools)
				}
			} else {
				if len(tc.req.Agent.Tools) != tc.wantTools {
					t.Errorf("len(Tools) = %d, want %d", len(tc.req.Agent.Tools), tc.wantTools)
				}
			}
		})
	}
}

func TestErrRequestActive(t *testing.T) {
	if ErrRequestActive == nil {
		t.Fatal("ErrRequestActive should not be nil")
	}
	if !errors.Is(ErrRequestActive, ErrRequestActive) {
		t.Error("errors.Is should match ErrRequestActive")
	}
}
