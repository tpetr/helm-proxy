package transcode

import (
	"encoding/json"
	"net/http"

	"k8s.io/helm/pkg/proto/hapi/services"
	"github.com/grpc-ecosystem/grpc-gateway/utilities"
	"github.com/grpc-ecosystem/grpc-gateway/runtime"
)

// Get retrieves a release record.
func (p *Proxy) Get(w http.ResponseWriter, r *http.Request) error {
	data, err := body(r)
	if err != nil {
		return err
	}

	req := &services.GetReleaseContentRequest{}

	runtime.PopulateQueryParameters(req, r.URL.Query(), utilities.NewDoubleArray(nil))

	var res *services.GetReleaseContentResponse
	err = p.do(func(rlc services.ReleaseServiceClient) error {
		ctx := NewContext()
		var err error
		res, err = rlc.GetReleaseContent(ctx, req)
		if err != nil {
			return err
		}
		return err
	})

	if err != nil {
		return err
	}

	data, err = json.Marshal(res)
	if err != nil {
		return err
	}

	_, err = w.Write(data)
	return err
}
