package server

import (
	"encoding/json"
	"io/ioutil"
	"net/http"

	"github.com/fluxcd/pkg/recorder"
)

func (s *HTTPServer) handleEvent() func(w http.ResponseWriter, r *http.Request) {
	return func(w http.ResponseWriter, r *http.Request) {
		body, err := ioutil.ReadAll(r.Body)
		if err != nil {
			s.logger.Error(err, "reading the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		defer r.Body.Close()

		payload := &recorder.Event{}
		err = json.Unmarshal(body, payload)
		if err != nil {
			s.logger.Error(err, "decoding the request body failed")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		w.WriteHeader(http.StatusAccepted)
	}
}
