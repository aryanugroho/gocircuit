package api

import (
	"bytes"
	"encoding/json"
	"io/ioutil"
	"net"
	"net/http"
	"strconv"
	"time"
)

type httpServer struct {
	server http.Server
}

type RespondFunc func(req []interface{}) []interface{}

func startServer(port int, respondAdd, respondSum RespondFunc) (*httpServer, error) {
	listener, err := net.Listen("tcp", ":" + strconv.Itoa(port))
	if err != nil {
		return nil, err
	}

	x := &httpServer{
		server: http.Server{
			ReadTimeout:    10*time.Second,
			WriteTimeout:   10*time.Second,
			MaxHeaderBytes: 20e3,
		},
	}

	serveMux := http.NewServeMux()
	x.server.Handler = serveMux

	serveMux.Handle("/v0/add", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { 
		handler(ReadAddRequestBatch, w, r, respondAdd)
	}))
	serveMux.Handle("/v0/sum", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { 
		handler(ReadSumRequestBatch, w, r, respondSum)
	}))

	go x.server.Serve(listener)

	return x, nil
}

// handler decodes an API batch request, []*???Request, from the body of an HTTP request, using the read function.
// It executes the request against the database using the respond function.
// Finally, it encodes the response to w.
func handler(read ReadRequestBatchFunc, w http.ResponseWriter, r *http.Request, respond RespondFunc) {
	if r.Body == nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("emtpy body"))
		return
	}
	defer r.Body.Close()

	// Pre-read the body
	buf, err := ioutil.ReadAll(r.Body)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("read request i/o: " + err.Error()))
		return
	}

	req, err := read(bytes.NewBuffer(buf))
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("read request: " + err.Error()))
		return
	}
	resp, err := RespondWithoutPanic(respond, req)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		w.Write([]byte("response: " + err.Error()))
		return
	}
	h := w.Header()
	h.Add("Content-Type", "application/json")
	h.Add("Access-Control-Allow-Origin", "*")
	h.Add("Access-Control-Allow-Methods", "GET, POST, OPTIONS")

	var bb bytes.Buffer
	enc := json.NewEncoder(&bb)
	for _, r := range resp {
		if err := enc.Encode(r); err != nil {
			panic("marshal response")
		}
	}
	w.Write(bb.Bytes())
}

func RespondWithoutPanic(f RespondFunc, a []interface{}) (r []interface{}, err error) {
	defer func() {
		if p := recover(); p != nil {
			r, err = nil, ErrBackend
		}
	}()
	r = f(a)
	return
}