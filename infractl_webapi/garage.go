package infractl_webapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/babbage88/infra-cli/infractl_services"
	coredeploy "github.com/babbage88/infra-core/deployment"
)

func (s *Server) createGarageTokenHandler(w http.ResponseWriter, r *http.Request) {
	defaultReq := infractl_services.DefaultGarageTokenRequest()
	defaultReq.SSH = s.defaultSSH

	req := coredeploy.GarageTokenRequest{}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
	}
	req = infractl_services.MergeGarageTokenDefaults(req, defaultReq)

	result, err := infractl_services.CreateGarageToken(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (s *Server) deployGarageNodeHandler(w http.ResponseWriter, r *http.Request) {
	defaultReq := infractl_services.DefaultGarageNodeRequest()
	defaultReq.SSH = s.defaultSSH

	req := coredeploy.GarageNodeRequest{}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
	}
	req = infractl_services.MergeGarageNodeDefaults(req, defaultReq)

	result, err := infractl_services.DeployGarageNode(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}
