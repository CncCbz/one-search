package api

import (
	"encoding/json"
	"net/http"
	"strconv"

	"github.com/go-chi/chi/v5"
	"github.com/one-search/one-search/backend/internal/model"
	"github.com/one-search/one-search/backend/internal/search"
)

func (h *Handler) keyQuota(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid id")
		return
	}
	var req model.ProviderKeyQuotaRequest
	_ = json.NewDecoder(r.Body).Decode(&req)
	key, err := h.store.GetAPIKeyByID(r.Context(), id)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	response, err := search.QueryOfficialQuota(r.Context(), key, req)
	if err != nil {
		writeError(w, http.StatusBadGateway, err.Error())
		return
	}
	if err := h.store.UpdateProviderKeyOfficialQuota(r.Context(), id, response); err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}
	h.audit(r, "admin", "provider_key.quota", "provider_key", strconv.FormatInt(id, 10), map[string]interface{}{"provider": response.Provider, "status": response.Status, "unit": response.Unit})
	writeJSON(w, http.StatusOK, response)
}
