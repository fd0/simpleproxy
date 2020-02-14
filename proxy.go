package main

import (
	"io"
	"net"
	"net/http"

	"golang.org/x/sync/errgroup"
)

type proxy struct {
	client *http.Client
}

func (p *proxy) ServeHTTP(rw http.ResponseWriter, req *http.Request) {
	if req.Method == http.MethodConnect {
		p.serveConnect(rw, req)
		return
	}

	outReq, err := http.NewRequestWithContext(req.Context(), req.Method, req.URL.String(), req.Body)
	if err != nil {
		panic(err)
	}

	// copy headers to outgoing request
	for name, values := range req.Header {
		outReq.Header.Del(name)
		for _, val := range values {
			outReq.Header.Add(name, val)
		}
	}

	outRes, err := p.client.Do(outReq)
	if err != nil {
		panic(err)
	}

	logger.Printf("%v %v -> %v", req.Method, req.URL, outRes.StatusCode)

	defer outRes.Body.Close()

	// copy response headers
	for name, values := range outRes.Header {
		rw.Header().Del(name)
		for _, val := range values {
			rw.Header().Add(name, val)
		}
	}

	rw.WriteHeader(outRes.StatusCode)

	_, err = io.Copy(rw, outRes.Body)
	if err != nil {
		panic(err)
	}
}

func (p *proxy) serveConnect(responseWriter http.ResponseWriter, req *http.Request) {
	logger.Printf("connect to %v", req.URL.Host)

	hj, ok := responseWriter.(http.Hijacker)
	if !ok {
		panic("unable to reuse connection for CONNECT")
	}

	conn, rw, err := hj.Hijack()
	if err != nil {
		panic(err)
	}

	port := req.URL.Port()
	if port == "" {
		switch req.URL.Scheme {
		case "http":
			port = "80"
		case "https":
			port = "443"
		default:
			panic("unable to determine port")
		}
	}
	target := net.JoinHostPort(req.URL.Hostname(), port)
	outConn, err := net.Dial("tcp", target)
	if err != nil {
		logger.Printf("connect to %v failed, %v", target, err)

		res := http.Response{
			Proto:         "HTTP/1.0",
			ProtoMajor:    1,
			ProtoMinor:    0,
			Status:        "unable to establish connection",
			StatusCode:    http.StatusInternalServerError,
			ContentLength: -1,
		}

		res.Write(rw)
		rw.Flush()

		return
	}

	defer outConn.Close()

	// write connect success
	res := http.Response{
		Proto:         "HTTP/1.0",
		ProtoMajor:    1,
		ProtoMinor:    0,
		Status:        "Connection Established by Proxy",
		StatusCode:    http.StatusOK,
		ContentLength: -1,
	}

	err = res.Write(rw)
	if err != nil {
		panic(err)
	}

	err = rw.Flush()
	if err != nil {
		conn.Close()
		panic(err)
	}

	var wg errgroup.Group

	wg.Go(func() error {
		_, err := io.Copy(outConn, rw)
		return err
	})

	wg.Go(func() error {
		_, err := io.Copy(rw, outConn)
		return err
	})

	err = wg.Wait()
	if err != nil {
		panic(err)
	}
}
