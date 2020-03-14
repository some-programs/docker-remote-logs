package agent

import (
	"encoding/json"
	"net/http"
)

func (h *handler) listContainers(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	containers, err := h.client.ListContainers(ctx)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	err = json.NewEncoder(w).Encode(containers)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}
