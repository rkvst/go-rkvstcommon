package httpserver

import (
	"net/http"
	"net/http/pprof"
	_ "net/http/pprof"

	"github.com/datatrails/go-datatrails-common/environment"
)

func NewPPROF(log Logger, name string) *Server {
	port, err := environment.GetRequired("PPROF_PORT")
	if err != nil {
		return nil
	}
	h := http.NewServeMux()
	h.HandleFunc("/debug/pprof/", pprof.Index)
	h.HandleFunc("/debug/pprof/cmdline", pprof.Cmdline)
	h.HandleFunc("/debug/pprof/profile", pprof.Profile)
	h.HandleFunc("/debug/pprof/symbol", pprof.Symbol)
	h.HandleFunc("/debug/pprof/trace", pprof.Trace)
	return New(log, name, port, h)
}
