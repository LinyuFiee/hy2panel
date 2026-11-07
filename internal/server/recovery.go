package server

import "net/http"

type recoveryRequest struct {
	AutoPort bool `json:"auto_port"`
}

func (s *Server) updateRecovery(w http.ResponseWriter, r *http.Request) {
	if !validMutation(r) {
		writeError(w, http.StatusForbidden, "请求来源无效")
		return
	}
	var input recoveryRequest
	if err := decodeJSON(w, r, &input); err != nil {
		writeError(w, http.StatusBadRequest, "恢复策略请求无效")
		return
	}
	current, err := s.store.Load()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "读取配置失败")
		return
	}
	next := *current
	next.Recovery.AutoPort = input.AutoPort
	if err := next.Validate(); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.store.Save(&next); err != nil {
		writeError(w, http.StatusInternalServerError, "保存恢复策略失败")
		return
	}
	writeJSON(w, http.StatusOK, s.makeState(r.Context(), &next))
}
