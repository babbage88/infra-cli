package infractl_webapi

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"

	"github.com/babbage88/infra-cli/infractl_services"
	coredeploy "github.com/babbage88/infra-core/deployment"
)

func (s *Server) installValkeyHandler(w http.ResponseWriter, r *http.Request) {
	defaultReq := infractl_services.DefaultValkeyInstallRequest()
	defaultReq.SSH = s.defaultSSH

	req := coredeploy.ValkeyInstallRequest{}
	if r.Body != nil {
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil && !errors.Is(err, io.EOF) {
			writeError(w, http.StatusBadRequest, "invalid JSON request body")
			return
		}
	}
	req = infractl_services.MergeValkeyInstallDefaults(req, defaultReq)

	result, err := infractl_services.InstallValkey(req)
	if err != nil {
		writeError(w, http.StatusInternalServerError, err.Error())
		return
	}

	writeJSON(w, http.StatusOK, result)
}
