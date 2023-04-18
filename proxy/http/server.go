package http

import (
	"net/http"
	"strings"

	"v2ray.com/core/app/proxyman"
	"v2ray.com/core/common/buffer"
	"v2ray.com/core/common/bufio"
	"v2ray.com/core/common/io"
	"v2ray.com/core/common/net"
	http_proto "v2ray.com/core/common/protocol/http"
	"v2ray.com/core/common/session"
	"v2ray.com/core/common/task"
	"v2ray.com/core/proxy"
)

var (
	errWaitAnother = newError("keep alive")
)

type server struct {
}

func NewServer() proxy.Server {
	return &server{}
}

func (s *server) Process(content session.Content, conn net.Conn, dispatcher proxyman.Dispatcher) error {
	requestReader := bufio.NewReader(conn)
	request, err := http.ReadRequest(requestReader)
	if err != nil {
		return newError("failed to read http request").WithError(err)
	}

	dst, err := func() (net.Address, error) {
		ib, _ := content.GetInbound()

		address := http_proto.ParseHostPort(request.URL.Host, request.URL.Scheme)
		return net.ParseAddress(ib.Source.Network.This(), address)
	}()
	if err != nil {
		return err
	}

	newError("receiving request [%s] [%s] [%s]", conn.RemoteAddr().String(), request.Method, request.URL.String()).AtInfo().Logging()

handle:
	if request.Method == http.MethodConnect {
		return handleConnect(content, dst, conn, dispatcher, request, requestReader)
	}

	err = handlePlainHTTP(content, dst, conn, dispatcher, request)
	if err == errWaitAnother {
		if strings.TrimSpace(strings.ToLower(request.Header.Get("Client-Connection"))) == "keep-alive" {
			goto handle
		}

		err = nil
	}

	return err
}

func handleConnect(content session.Content, dst net.Address, conn net.Conn, dispatcher proxyman.Dispatcher, _ *http.Request, requestReader *bufio.Reader) error {
	connWriter, connReader := buffer.NewAllToBytesWriter(conn), buffer.NewIOReader(conn)

	_, err := conn.Write([]byte("HTTP/1.1 200 Connection established\r\n\r\n"))
	if err != nil {
		return newError("failed to write back OK response").WithError(err)
	}

	link, err := dispatcher.Dispatch(content, dst)
	if err != nil {
		return err
	}

	if requestReader.Buffered() > 0 {
		defer func() {
			requestReader = nil
		}()

		payload, err := buffer.ReadMultiFrom(io.LimitReader(requestReader, int64(requestReader.Buffered())))
		if err != nil {
			return err
		}

		if err := link.Writer.WriteMultiBuffer(payload); err != nil {
			return err
		}
	}

	requestDone := func() error {
		defer func() {
			_ = link.Writer.Close()
		}()

		return buffer.Copy(link.Writer, connReader)
	}

	responseDone := func() error {
		return buffer.Copy(connWriter, link.Reader)
	}

	if errs := task.Parallel(requestDone, responseDone); len(errs) > 0 {
		return newError("connection ends").WithError(errs)
	}
	return nil
}

func handlePlainHTTP(content session.Content, dst net.Address, conn net.Conn, dispatcher proxyman.Dispatcher, request *http.Request) error {
	http_proto.RemoveHopByHopHeaders(request.Header)

	// Prevent UA from being set to golang's default ones
	if len(request.Header.Get("User-Agent")) == 0 {
		request.Header.Set("User-Agent", "")
	}

	link, err := dispatcher.Dispatch(content, dst)
	if err != nil {
		return err
	}

	result := error(errWaitAnother)

	requestDone := func() error {
		defer func() {
			_ = link.Writer.Close()
		}()

		request.Header.Set("Connection", "close")

		requestWriter := buffer.NewBufferedWriter(link.Writer)

		if err := requestWriter.SetBuffered(false); err != nil {
			return err
		}

		return request.Write(requestWriter)
	}

	responseDone := func() error {
		responseReader := bufio.NewReader(buffer.NewBufferedReader(link.Reader))

		response, err := http.ReadResponse(responseReader, request)
		if err == nil {
			defer func() {
				_ = response.Body.Close()
			}()

			http_proto.RemoveHopByHopHeaders(response.Header)

			if response.ContentLength >= 0 {
				response.Header.Set("Client-Connection", "keep-alive")
				response.Header.Set("Connection", "keep-alive")
				response.Header.Set("Keep-Alive", "timeout=4")
				response.Close = false
			} else {
				response.Close = true
				result = nil
			}
		} else {
			newError("failed to read response from %s", request.URL.Host).WithError(err).AtDebug().Logging()

			response = &http.Response{
				Status:        "Service Unavailable",
				StatusCode:    503,
				Proto:         "HTTP/1.1",
				ProtoMajor:    1,
				ProtoMinor:    1,
				Header:        make(map[string][]string, 2),
				Body:          nil,
				ContentLength: 0,
				Close:         true,
			}
			response.Header.Set("Connection", "close")
			response.Header.Set("Client-Connection", "close")
		}

		return response.Write(conn)
	}

	if errs := task.Parallel(requestDone, responseDone); len(errs) > 0 {
		return newError("connection ends").WithError(errs)
	}

	return result
}
