package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log/slog"
	"maps"
	"net"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"
)

type statusRecorder struct {
	http.ResponseWriter

	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func logger(handler http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: 502}

		handler(rec, r)

		host, _, _ := net.SplitHostPort(r.RemoteAddr)
		slog.Info(
			"access",
			"method", r.Method,
			"path", r.URL.Path,
			"host", host,
			"status", rec.status,
			"duration_ms", time.Since(start).Milliseconds(),
		)
	}
}

type VM struct {
	Hostname string `json:"hostname"`
	IP       string `json:"ip"`
}

type Registry struct {
	mutex sync.RWMutex
	vms   map[string]VM
}

func NewRegistry() *Registry {
	return &Registry{
		mutex: sync.RWMutex{},
		vms:   make(map[string]VM),
	}
}

func (r *Registry) Upsert(hostname, ip string) VM {
	r.mutex.Lock()
	defer r.mutex.Unlock()

	vm := VM{Hostname: hostname, IP: ip}
	r.vms[hostname] = vm

	return vm
}

func (r *Registry) List() map[string]VM {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	vms := make(map[string]VM, len(r.vms))
	maps.Copy(vms, r.vms)

	return vms
}

func (r *Registry) Hostnames() []string {
	r.mutex.RLock()
	defer r.mutex.RUnlock()

	hostnames := make([]string, 0, len(r.vms))
	for name := range r.vms {
		hostnames = append(hostnames, name)
	}

	return hostnames
}

func WriteJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)

	_ = json.NewEncoder(w).Encode(v)
}

var reg = NewRegistry()

func health(w http.ResponseWriter, _ *http.Request) {
	WriteJSON(w, 200, map[string]string{"status": "ok", "vms": strings.Join(reg.Hostnames(), ",")})
}

func inventory(w http.ResponseWriter, _ *http.Request) {
	vms := reg.List()
	hosts := make([]string, 0, len(vms))
	hostvars := make(map[string]map[string]string, len(vms))

	for name, vm := range vms {
		hosts = append(hosts, name)
		hostvars[name] = map[string]string{"ansible_host": vm.IP}
	}

	WriteJSON(w, 200, map[string]any{
		"all": map[string]any{
			"hosts": hosts,
		},
		"_meta": map[string]any{
			"hostvars": hostvars,
		},
	})
}

func checkin(w http.ResponseWriter, req *http.Request) {
	err := req.ParseForm()
	if err != nil {
		WriteJSON(w, 400, map[string]string{"error": "bad request"})

		return
	}

	hostname := req.FormValue("hostname")
	if hostname == "" {
		WriteJSON(w, 400, map[string]string{"error": "hostname required"})

		return
	}

	ip := req.RemoteAddr

	idx := strings.LastIndex(ip, ":")
	if idx != -1 {
		ip = ip[:idx]
	}

	WriteJSON(w, 201, reg.Upsert(hostname, ip))
}

func main() {
	port := flag.Int("port", 7890, "listen port")

	flag.Parse()

	mux := http.NewServeMux()
	mux.HandleFunc("GET /", logger(health))
	mux.HandleFunc("GET /inventory", logger(inventory))
	mux.HandleFunc("POST /inventory", logger(checkin))

	addr := fmt.Sprintf("0.0.0.0:%d", *port)
	slog.Info("listening on " + addr)

	err := http.ListenAndServe(addr, mux)
	if err != nil {
		slog.Error("server failed: " + err.Error())
		os.Exit(1)
	}
}
