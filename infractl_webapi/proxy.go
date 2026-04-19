package infractl_webapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/babbage88/infra-cli/infractl_services"
	coredeploy "github.com/babbage88/infra-core/deployment"
)

func (s *Server) installProxyHandler(w http.ResponseWriter, r *http.Request) {
	proxyName := r.PathValue("name")
	defaultReq, err := infractl_services.DefaultProxyInstallRequest(proxyName)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	defaultReq.SSH = s.defaultSSH

	req := coredeploy.ProxyInstallRequest{}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
	}
	req = infractl_services.MergeProxyInstallDefaults(req, defaultReq)

	result, err := infractl_services.InstallProxy(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}
