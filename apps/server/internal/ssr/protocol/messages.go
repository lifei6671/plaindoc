package protocol

import "encoding/json"

// MessageType 描述 Go 与 SSR Worker 间的请求类型。
type MessageType string

const (
	// MessageTypeHandshake 表示 Worker 启动后的协议握手。
	MessageTypeHandshake MessageType = "handshake"
	// MessageTypeRender 表示执行一次页面渲染。
	MessageTypeRender MessageType = "render"
)

// HandshakeRequest 用于 Worker 启动时的协议版本协商。
type HandshakeRequest struct {
	Type    MessageType `json:"type"`
	Version string      `json:"version"`
}

// HandshakeResponse 返回 Worker 当前协议版本与握手结果。
type HandshakeResponse struct {
	Type    MessageType  `json:"type"`
	OK      bool         `json:"ok"`
	Version string       `json:"version"`
	Error   *RenderError `json:"error,omitempty"`
}

// RenderRequest 是 Go -> Worker 的渲染请求。
type RenderRequest struct {
	ID         string          `json:"id"`
	Type       MessageType     `json:"type"`
	Version    string          `json:"version"`
	Route      string          `json:"route"`
	DeadlineMS int64           `json:"deadlineMs"`
	Payload    json.RawMessage `json:"payload"`
}

// RenderHead 携带页面 head 相关信息。
type RenderHead struct {
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Canonical   string `json:"canonical,omitempty"`
}

// RenderMetrics 记录一次渲染的关键指标。
type RenderMetrics struct {
	RenderMS     int64 `json:"renderMs,omitempty"`
	PayloadBytes int64 `json:"payloadBytes,omitempty"`
}

// RenderError 描述 Worker 渲染失败原因。
type RenderError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

// RenderResponse 是 Worker -> Go 的渲染结果。
type RenderResponse struct {
	ID      string        `json:"id"`
	OK      bool          `json:"ok"`
	HTML    string        `json:"html,omitempty"`
	Head    RenderHead    `json:"head,omitempty"`
	Error   *RenderError  `json:"error,omitempty"`
	Metrics RenderMetrics `json:"metrics,omitempty"`
}
