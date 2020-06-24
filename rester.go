package rester

import (
	"net"
	"os"
	"sync"
	"time"

	"github.com/valyala/fasthttp"
)

// alias
type (
	// RequestHandler must process incoming requests.
	//
	// RequestHandler must call ctx.TimeoutError() before returning
	// if it keeps references to ctx and/or its' members after the return.
	// Consider wrapping RequestHandler into TimeoutHandler if response time
	// must be limited.
	RequestHandler = fasthttp.RequestHandler

	// RequestCtx contains incoming request and manages outgoing response.
	//
	// It is forbidden copying RequestCtx instances.
	//
	// RequestHandler should avoid holding references to incoming RequestCtx and/or
	// its' members after the return.
	// If holding RequestCtx references after the return is unavoidable
	// (for instance, ctx is passed to a separate goroutine and ctx lifetime cannot
	// be controlled), then the RequestHandler MUST call ctx.TimeoutError()
	// before return.
	//
	// It is unsafe modifying/reading RequestCtx instance from concurrently
	// running goroutines. The only exception is TimeoutError*, which may be called
	// while other goroutines accessing RequestCtx.
	RequestCtx = fasthttp.RequestCtx

	// Logger is used for logging formatted messages.
	Logger = fasthttp.Logger

	// RequestHeader represents HTTP request header.
	//
	// It is forbidden copying RequestHeader instances.
	// Create new instances instead and use CopyTo.
	//
	// RequestHeader instance MUST NOT be used from concurrently running
	// goroutines.
	RequestHeader = fasthttp.RequestHeader

	// RequestConfig configure the per request deadline and body limits
	RequestConfig = fasthttp.RequestConfig

	// A ConnState represents the state of a client connection to a server.
	// It's used by the optional Server.ConnState hook.
	ConnState = fasthttp.ConnState
)

// Engine is the framework's instance, it contains the muxer, middleware and configuration settings.
// Create an instance of Engine, by using New() or Default()
type Engine struct {
	Router

	// Enables automatic redirection if the current route can't be matched but a
	// handler for the path with (without) the trailing slash exists.
	// For example if /foo/ is requested but a route only exists for /foo, the
	// client is redirected to /foo with http status code 301 for GET requests
	// and 307 for all other request methods.
	RedirectTrailingSlash bool

	// If enabled, the router tries to fix the current request path, if no
	// handle is registered for it.
	// First superfluous path elements like ../ or // are removed.
	// Afterwards the router does a case-insensitive lookup of the cleaned path.
	// If a handle can be found for this route, the router makes a redirection
	// to the corrected path with status code 301 for GET requests and 307 for
	// all other request methods.
	// For example /FOO and /..//Foo could be redirected to /foo.
	// RedirectTrailingSlash is independent of this option.
	RedirectFixedPath bool

	// If enabled, the router checks if another method is allowed for the
	// current route, if the current request can not be routed.
	// If this is the case, the request is answered with 'Method Not Allowed'
	// and HTTP status code 405.
	// If no other Method is allowed, the request is delegated to the NotFound
	// handler.
	HandleMethodNotAllowed bool

	// If enabled, the router automatically replies to OPTIONS requests.
	// Custom OPTIONS handlers take priority over automatic replies.
	HandleOPTIONS bool

	// Configurable http.Handler which is called when no matching route is
	// found. If it is not set, http.NotFound is used.
	NotFound fasthttp.RequestHandler

	// Configurable http.Handler which is called when a request
	// cannot be routed and HandleMethodNotAllowed is true.
	// If it is not set, http.Error with http.StatusMethodNotAllowed is used.
	// The "Allow" header with allowed request methods is set before the handler
	// is called.
	MethodNotAllowed fasthttp.RequestHandler

	// Function to handle panics recovered from http handlers.
	// It should be used to generate a error page and return the http error code
	// 500 (Internal Server Error).
	// The handler can be used to keep your server from crashing because of
	// unrecovered panics.
	PanicHandler func(*fasthttp.RequestCtx, interface{})

	// -------------- server ----------------

	server fasthttp.Server

	// ErrorHandler for returning a response in case of an error while receiving or parsing the request.
	//
	// The following is a non-exhaustive list of errors that can be expected as argument:
	//   * io.EOF
	//   * io.ErrUnexpectedEOF
	//   * ErrGetOnly
	//   * ErrSmallBuffer
	//   * ErrBodyTooLarge
	//   * ErrBrokenChunks
	ErrorHandler func(ctx *RequestCtx, err error)

	// HeaderReceived is called after receiving the header
	//
	// non zero RequestConfig field values will overwrite the default configs
	HeaderReceived func(header *RequestHeader) RequestConfig

	// ContinueHandler is called after receiving the Expect 100 Continue Header
	//
	// https://www.w3.org/Protocols/rfc2616/rfc2616-sec8.html#sec8.2.3
	// https://www.w3.org/Protocols/rfc2616/rfc2616-sec10.html#sec10.1.1
	// Using ContinueHandler a server can make decisioning on whether or not
	// to read a potentially large request body based on the headers
	//
	// The default is to automatically read request bodies of Expect 100 Continue requests
	// like they are normal requests
	ContinueHandler func(header *RequestHeader) bool

	// Server name for sending in response headers.
	//
	// Default server name is used if left blank.
	Name string

	// The maximum number of concurrent connections the server may serve.
	//
	// DefaultConcurrency is used if not set.
	//
	// Concurrency only works if you either call Serve once, or only ServeConn multiple times.
	// It works with ListenAndServe as well.
	Concurrency int

	// Whether to disable keep-alive connections.
	//
	// The server will close all the incoming connections after sending
	// the first response to client if this option is set to true.
	//
	// By default keep-alive connections are enabled.
	DisableKeepalive bool

	// Per-connection buffer size for requests' reading.
	// This also limits the maximum header size.
	//
	// Increase this buffer if your clients send multi-KB RequestURIs
	// and/or multi-KB headers (for example, BIG cookies).
	//
	// Default buffer size is used if not set.
	ReadBufferSize int

	// Per-connection buffer size for responses' writing.
	//
	// Default buffer size is used if not set.
	WriteBufferSize int

	// ReadTimeout is the amount of time allowed to read
	// the full request including body. The connection's read
	// deadline is reset when the connection opens, or for
	// keep-alive connections after the first byte has been read.
	//
	// By default request read timeout is unlimited.
	ReadTimeout time.Duration

	// WriteTimeout is the maximum duration before timing out
	// writes of the response. It is reset after the request handler
	// has returned.
	//
	// By default response write timeout is unlimited.
	WriteTimeout time.Duration

	// IdleTimeout is the maximum amount of time to wait for the
	// next request when keep-alive is enabled. If IdleTimeout
	// is zero, the value of ReadTimeout is used.
	IdleTimeout time.Duration

	// Maximum number of concurrent client connections allowed per IP.
	//
	// By default unlimited number of concurrent connections
	// may be established to the server from a single IP address.
	MaxConnsPerIP int

	// Maximum number of requests served per connection.
	//
	// The server closes connection after the last request.
	// 'Connection: close' header is added to the last response.
	//
	// By default unlimited number of requests may be served per connection.
	MaxRequestsPerConn int

	// Whether to enable tcp keep-alive connections.
	//
	// Whether the operating system should send tcp keep-alive messages on the tcp connection.
	//
	// By default tcp keep-alive connections are disabled.
	TCPKeepalive bool

	// Period between tcp keep-alive messages.
	//
	// TCP keep-alive period is determined by operation system by default.
	TCPKeepalivePeriod time.Duration

	// Maximum request body size.
	//
	// The server rejects requests with bodies exceeding this limit.
	//
	// Request body size is limited by DefaultMaxRequestBodySize by default.
	MaxRequestBodySize int

	// Aggressively reduces memory usage at the cost of higher CPU usage
	// if set to true.
	//
	// Try enabling this option only if the server consumes too much memory
	// serving mostly idle keep-alive connections. This may reduce memory
	// usage by more than 50%.
	//
	// Aggressive memory usage reduction is disabled by default.
	ReduceMemoryUsage bool

	// Rejects all non-GET requests if set to true.
	//
	// This option is useful as anti-DoS protection for servers
	// accepting only GET requests. The request size is limited
	// by ReadBufferSize if GetOnly is set.
	//
	// Server accepts all the requests by default.
	GetOnly bool

	// Will not pre parse Multipart Form data if set to true.
	//
	// This option is useful for servers that desire to treat
	// multipart form data as a binary blob, or choose when to parse the data.
	//
	// Server pre parses multipart form data by default.
	DisablePreParseMultipartForm bool

	// Logs all errors, including the most frequent
	// 'connection reset by peer', 'broken pipe' and 'connection timeout'
	// errors. Such errors are common in production serving real-world
	// clients.
	//
	// By default the most frequent errors such as
	// 'connection reset by peer', 'broken pipe' and 'connection timeout'
	// are suppressed in order to limit output log traffic.
	LogAllErrors bool

	// Header names are passed as-is without normalization
	// if this option is set.
	//
	// Disabled header names' normalization may be useful only for proxying
	// incoming requests to other servers expecting case-sensitive
	// header names. See https://github.com/valyala/fasthttp/issues/57
	// for details.
	//
	// By default request and response header names are normalized, i.e.
	// The first letter and the first letters following dashes
	// are uppercased, while all the other letters are lowercased.
	// Examples:
	//
	//     * HOST -> Host
	//     * content-type -> Content-Type
	//     * cONTENT-lenGTH -> Content-Length
	DisableHeaderNamesNormalizing bool

	// SleepWhenConcurrencyLimitsExceeded is a duration to be slept of if
	// the concurrency limit in exceeded (default [when is 0]: don't sleep
	// and accept new connections immidiatelly).
	SleepWhenConcurrencyLimitsExceeded time.Duration

	// NoDefaultServerHeader, when set to true, causes the default Server header
	// to be excluded from the Response.
	//
	// The default Server header value is the value of the Name field or an
	// internal default value in its absence. With this option set to true,
	// the only time a Server header will be sent is if a non-zero length
	// value is explicitly provided during a request.
	NoDefaultServerHeader bool

	// NoDefaultDate, when set to true, causes the default Date
	// header to be excluded from the Response.
	//
	// The default Date header value is the current date value. When
	// set to true, the Date will not be present.
	NoDefaultDate bool

	// NoDefaultContentType, when set to true, causes the default Content-Type
	// header to be excluded from the Response.
	//
	// The default Content-Type header value is the internal default value. When
	// set to true, the Content-Type will not be present.
	NoDefaultContentType bool

	// ConnState specifies an optional callback function that is
	// called when a client connection changes state. See the
	// ConnState type and associated constants for details.
	ConnState func(net.Conn, ConnState)

	// Logger, which is used by RequestCtx.Logger().
	//
	// By default standard logger from log package is used.
	Logger Logger

	// KeepHijackedConns is an opt-in disable of connection
	// close by fasthttp after connections' HijackHandler returns.
	// This allows to save goroutines, e.g. when fasthttp used to upgrade
	// http connections to WS and connection goes to another handler,
	// which will close it when needed.
	KeepHijackedConns bool

	once sync.Once
}

// New returns a new blank Engine instance.
// By default the configuration is:
// - RedirectTrailingSlash:  true
// - RedirectFixedPath:      false
// - HandleMethodNotAllowed: false
// - HandleOPTIONS:          true
func New() *Engine {
	engine := &Engine{
		RedirectTrailingSlash:  true,
		RedirectFixedPath:      false,
		HandleMethodNotAllowed: false,
		HandleOPTIONS:          true,
	}
	return engine
}

func (engine *Engine) initOnce() {
	engine.once.Do(func() {
		// router
		engine.Router.router.RedirectTrailingSlash = engine.RedirectTrailingSlash
		engine.Router.router.RedirectFixedPath = engine.RedirectFixedPath
		engine.Router.router.HandleMethodNotAllowed = engine.HandleMethodNotAllowed
		engine.Router.router.HandleOPTIONS = engine.HandleOPTIONS
		engine.Router.router.NotFound = engine.NotFound
		engine.Router.router.MethodNotAllowed = engine.MethodNotAllowed
		engine.Router.router.PanicHandler = engine.PanicHandler
		// server
		engine.server.Handler = engine.Router.router.Handler
		engine.server.ErrorHandler = engine.ErrorHandler
		engine.server.HeaderReceived = engine.HeaderReceived
		engine.server.ContinueHandler = engine.ContinueHandler
		engine.server.Name = engine.Name
		engine.server.Concurrency = engine.Concurrency
		engine.server.DisableKeepalive = engine.DisableKeepalive
		engine.server.ReadBufferSize = engine.ReadBufferSize
		engine.server.WriteBufferSize = engine.WriteBufferSize
		engine.server.ReadTimeout = engine.ReadTimeout
		engine.server.WriteTimeout = engine.WriteTimeout
		engine.server.IdleTimeout = engine.IdleTimeout
		engine.server.MaxConnsPerIP = engine.MaxConnsPerIP
		engine.server.MaxRequestsPerConn = engine.MaxRequestsPerConn
		engine.server.TCPKeepalive = engine.TCPKeepalive
		engine.server.TCPKeepalivePeriod = engine.TCPKeepalivePeriod
		engine.server.MaxRequestBodySize = engine.MaxRequestBodySize
		engine.server.ReduceMemoryUsage = engine.ReduceMemoryUsage
		engine.server.GetOnly = engine.GetOnly
		engine.server.DisablePreParseMultipartForm = engine.DisablePreParseMultipartForm
		engine.server.LogAllErrors = engine.LogAllErrors
		engine.server.DisableHeaderNamesNormalizing = engine.DisableHeaderNamesNormalizing
		engine.server.SleepWhenConcurrencyLimitsExceeded = engine.SleepWhenConcurrencyLimitsExceeded
		engine.server.NoDefaultServerHeader = engine.NoDefaultServerHeader
		engine.server.NoDefaultDate = engine.NoDefaultDate
		engine.server.NoDefaultContentType = engine.NoDefaultContentType
		engine.server.ConnState = engine.ConnState
		engine.server.Logger = engine.Logger
		engine.server.KeepHijackedConns = engine.KeepHijackedConns
	})
}

// ListenAndServe serves HTTP requests from the given TCP4 addr.
//
// Pass custom listener to Serve if you need listening on non-TCP4 media
// such as IPv6.
//
// Accepted connections are configured to enable TCP keep-alives.
func (engine *Engine) ListenAndServe(addr string) error {
	engine.initOnce()
	return engine.server.ListenAndServe(addr)
}

// ListenAndServeUNIX serves HTTP requests from the given UNIX addr.
//
// The function deletes existing file at addr before starting serving.
//
// The server sets the given file mode for the UNIX addr.
func (engine *Engine) ListenAndServeUNIX(addr string, mode os.FileMode) error {
	engine.initOnce()
	return engine.server.ListenAndServeUNIX(addr, mode)
}

// ListenAndServeTLS serves HTTPS requests from the given TCP4 addr.
//
// certFile and keyFile are paths to TLS certificate and key files.
//
// Pass custom listener to Serve if you need listening on non-TCP4 media
// such as IPv6.
//
// If the certFile or keyFile has not been provided to the server structure,
// the function will use the previously added TLS configuration.
//
// Accepted connections are configured to enable TCP keep-alives.
func (engine *Engine) ListenAndServeTLS(addr, certFile, keyFile string) error {
	engine.initOnce()
	return engine.server.ListenAndServeTLS(addr, certFile, keyFile)
}

// ListenAndServeTLSEmbed serves HTTPS requests from the given TCP4 addr.
//
// certData and keyData must contain valid TLS certificate and key data.
//
// Pass custom listener to Serve if you need listening on arbitrary media
// such as IPv6.
//
// If the certFile or keyFile has not been provided the server structure,
// the function will use previously added TLS configuration.
//
// Accepted connections are configured to enable TCP keep-alives.
func (engine *Engine) ListenAndServeTLSEmbed(addr string, certData, keyData []byte) error {
	engine.initOnce()
	return engine.server.ListenAndServeTLSEmbed(addr, certData, keyData)
}

// ServeTLS serves HTTPS requests from the given listener.
//
// certFile and keyFile are paths to TLS certificate and key files.
//
// If the certFile or keyFile has not been provided the server structure,
// the function will use previously added TLS configuration.
func (engine *Engine) ServeTLS(ln net.Listener, certFile, keyFile string) error {
	engine.initOnce()
	return engine.server.ServeTLS(ln, certFile, keyFile)
}

// ServeTLSEmbed serves HTTPS requests from the given listener.
//
// certData and keyData must contain valid TLS certificate and key data.
//
// If the certFile or keyFile has not been provided the server structure,
// the function will use previously added TLS configuration.
func (engine *Engine) ServeTLSEmbed(ln net.Listener, certData, keyData []byte) error {
	engine.initOnce()
	return engine.server.ServeTLSEmbed(ln, certData, keyData)
}

// Serve serves incoming connections from the given listener.
//
// Serve blocks until the given listener returns permanent error.
func (engine *Engine) Serve(ln net.Listener) error {
	engine.initOnce()
	return engine.server.Serve(ln)
}

// ServeConn serves HTTP requests from the given connection.
//
// ServeConn returns nil if all requests from the c are successfully served.
// It returns non-nil error otherwise.
//
// Connection c must immediately propagate all the data passed to Write()
// to the client. Otherwise requests' processing may hang.
//
// ServeConn closes c before returning.
func (engine *Engine) ServeConn(c net.Conn) error {
	engine.initOnce()
	return engine.server.ServeConn(c)
}

// Shutdown gracefully shuts down the server without interrupting any active connections.
// Shutdown works by first closing all open listeners and then waiting indefinitely for all connections to return to idle and then shut down.
//
// When Shutdown is called, Serve, ListenAndServe, and ListenAndServeTLS immediately return nil.
// Make sure the program doesn't exit and waits instead for Shutdown to return.
//
// Shutdown does not close keepalive connections so its recommended to set ReadTimeout to something else than 0.
func (engine *Engine) Shutdown() error {
	engine.initOnce()
	return engine.server.Shutdown()
}

// GetCurrentConcurrency returns a number of currently served
// connections.
//
// This function is intended be used by monitoring systems
func (engine *Engine) GetCurrentConcurrency() uint32 {
	engine.initOnce()
	return engine.server.GetCurrentConcurrency()
}

// GetOpenConnectionsCount returns a number of opened connections.
//
// This function is intended be used by monitoring systems
func (engine *Engine) GetOpenConnectionsCount() int32 {
	engine.initOnce()
	return engine.server.GetOpenConnectionsCount()
}
