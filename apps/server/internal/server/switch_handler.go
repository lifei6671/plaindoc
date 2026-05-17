package server

import (
	"net/http"
	"sync/atomic"
)

// SwitchHandler 用于启动期从 bootstrap handler 原子切换到正式业务 router。
type SwitchHandler struct {
	current atomic.Value
}

type switchHandlerValue struct {
	handler http.Handler
}

// NewSwitchHandler 创建可切换 handler。
func NewSwitchHandler(initial http.Handler) *SwitchHandler {
	handler := &SwitchHandler{}
	if initial == nil {
		initial = http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
			http.Error(w, errStartupHandlerNotSet.Error(), http.StatusServiceUnavailable)
		})
	}
	handler.current.Store(switchHandlerValue{handler: initial})
	return handler
}

// Set 原子替换当前 handler。
func (h *SwitchHandler) Set(next http.Handler) {
	if h == nil || next == nil {
		return
	}
	h.current.Store(switchHandlerValue{handler: next})
}

// ServeHTTP 将请求交给当前 handler。
func (h *SwitchHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if h == nil {
		http.Error(w, errStartupHandlerNotSet.Error(), http.StatusServiceUnavailable)
		return
	}
	current := h.current.Load()
	if current == nil {
		http.Error(w, errStartupHandlerNotSet.Error(), http.StatusServiceUnavailable)
		return
	}
	value, ok := current.(switchHandlerValue)
	if !ok || value.handler == nil {
		http.Error(w, errStartupHandlerNotSet.Error(), http.StatusServiceUnavailable)
		return
	}
	value.handler.ServeHTTP(w, r)
}
