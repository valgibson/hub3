package ikuzo

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"runtime/debug"
	"syscall"
	"time"

	"github.com/delving/hub3/ikuzo/logger"
	"github.com/delving/hub3/ikuzo/middleware"
	"github.com/delving/hub3/ikuzo/service/organization"
	"github.com/go-chi/chi"
	"github.com/rs/xid"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/hlog"
	"github.com/rs/zerolog/log"
)

const (
	defaultServerPort      = 3000
	defaultShutdownTimeout = 10
)

// Server provides a net/http compliant WebServer.
type Server interface {
	ListenAndServe() error
	ServeHTTP(w http.ResponseWriter, r *http.Request)
}

// ServiceCancellation must be implement by each service that uses background services or connections.
type ServiceCancellation interface {
	Shutdown(ctx context.Context) error
}

type server struct {
	// router is compatible with http.Mux
	router chi.Router
	// port is where the server will listen to TCP requests
	port int
	// cancelFunc is called for graceful shutdown of resources and background workers.
	cancelFunc context.CancelFunc
	// workers is a pool that manages all the background WorkerServices
	workers *workerPool
	// gracefulTimeout maximum duration of graceful shutdown of server. (default: 10 seconds)
	gracefulTimeout time.Duration
	// disableRequestLogger stops logging of request information to the global logger
	disableRequestLogger bool
	// loggerConfig is the logger configuration for the server
	loggerConfig logger.Config
	// middleware is an array of middleware options that will be applied.
	// When none are given the default middleware is applied.
	middleware []func(http.Handler) http.Handler
	// routerFuncs are the custom routes.
	// When none are given the default routes are applied.
	routerFuncs []RouterFunc
	// service to access the organization store
	organizations *organization.Service
}

// NewServer returns the default server.
// The configuration can be modified using Option functions.
// All services are lazy-loaded.
func NewServer(options ...Option) (Server, error) {
	return newServer(options...)
}

// newServer returns the default server.
func newServer(options ...Option) (*server, error) {
	ctx, cancelFunc := context.WithCancel(context.Background())
	s := &server{
		port:            defaultServerPort,
		cancelFunc:      cancelFunc,
		workers:         newWorkerPool(ctx),
		gracefulTimeout: defaultShutdownTimeout * time.Second,
	}

	s.setRouterdefaults()

	// apply options
	for _, option := range options {
		if err := option(s); err != nil {
			return nil, err
		}
	}

	// setting up logging
	log.Logger = logger.NewLogger(s.loggerConfig)

	// apply middleware
	if len(s.middleware) == 0 {
		s.middleware = DefaultMiddleware()
	}

	s.router.Use(s.middleware...)

	// recover is not optional
	s.router.Use(s.recoverer)

	// setting up request logging middleware
	if !s.disableRequestLogger {
		s.router.Use(middleware.RequestLogger(&log.Logger))
	}

	// setting default services
	s.setDefaultServices()

	// apply default routes
	s.routes()

	// apply custom routes
	for _, f := range s.routerFuncs {
		f(s.router)
	}

	// TODO: maybe add server validation function

	return s, nil
}

func (s *server) setDefaultServices() {
	if s.organizations == nil {
		s.organizations = organization.NewService(nil)

		log.Warn().
			Msg(
				"Starting organization.Service with in-memory mode. " +
					"Use ikuzo.SetOrganisationService to set a persistent service",
			)
	}
}

func (s *server) setRouterdefaults() {
	router := chi.NewRouter()
	router.NotFound(s.handle404)
	router.MethodNotAllowed(s.handleMethodNotAllowed)

	s.router = router
}

func (s *server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.router.ServeHTTP(w, r)
}

// ListenAndServe starts a HTTP-server with graceful shutdown.
func (s *server) ListenAndServe() error {
	return s.listenAndServe()
}

func (s *server) listenAndServe(testSignals ...interface{}) error {
	log.Info().
		Int("port", s.port).
		Msg("starting server")

	// gather errors
	allowedErrors := 10
	errChan := make(chan error, allowedErrors)

	// start web-server
	server := http.Server{Addr: fmt.Sprintf(":%d", s.port), Handler: s}

	go func() {
		errChan <- server.ListenAndServe()
	}()

	// watch for quit signals
	signalChan := make(chan os.Signal, 1)
	signal.Notify(signalChan, syscall.SIGINT, syscall.SIGTERM)

	// inject signals for testing
	for _, sign := range testSignals {
		switch v := sign.(type) {
		case os.Signal:
			signalChan <- v.(os.Signal)
		case error:
			errChan <- v.(error)
		}
	}

	// block until a select case is satisfied
	for {
		select {
		case err := <-errChan:
			if err != nil {
				return err
			}
		case sig := <-signalChan:
			log.Warn().
				Str("signal", sig.String()).
				Msg("caught shutdown signal, starting graceful shutdown")

			return s.shutdown(&server)
		case <-s.workers.ctx.Done():
			return s.workers.ctx.Err()
		}
	}
}

func (s *server) shutdown(server *http.Server) error {
	log.Info().Msg("sending stop signal to background processes")

	// cancel context to shutdown background processes and connections
	s.cancelFunc()

	// set maximum duration for graceful shutdown
	ctx, cancel := context.WithTimeout(context.Background(), s.gracefulTimeout)
	defer cancel()

	log.Info().Msg("stopping web-server")
	server.SetKeepAlivesEnabled(false)
	_ = server.Shutdown(ctx)

	// wait until all background workers are finished
	s.workers.wg.Wait()
	log.Info().Msg("finished shutting down background processes")

	return nil
}

// decode decodes the body of the http.Request into the provided interface.
func (s *server) decode(r *http.Request, v interface{}) error {
	return json.NewDecoder(r.Body).Decode(v)
}

// handle404 returns a custom response when a page is not found.
func (s *server) handle404(w http.ResponseWriter, r *http.Request) {
	s.respondWithError(w, r, errors.New("page not found"), http.StatusNotFound)
}

// handleIndex returns default information about the deployment
func (s *server) handleIndex() http.HandlerFunc {
	type response struct {
		Name    string `json:"name"`
		Version string `json:"version"`
		Message string `json:"message"`
	}

	return func(w http.ResponseWriter, r *http.Request) {
		s.respond(
			w,
			r,
			response{
				Name:    "Hub3",
				Version: "0.0.1",
				Message: "Ikuzo, Barbatos",
			},
			http.StatusOK,
		)
	}
}

// handleMethodNotAllowed returns a custom response when a method is not allowed.
func (s *server) handleMethodNotAllowed(w http.ResponseWriter, r *http.Request) {
	s.respondWithError(w, r, fmt.Errorf("method %s is not allowed", r.Method), http.StatusMethodNotAllowed)
}

// Get the logger from the request's context. You can safely assume it
// will be always there: if the handler is removed, hlog.FromRequest
// will return a no-op logger.
func (s *server) requestLogger(r *http.Request) *zerolog.Logger {
	return hlog.FromRequest(r)
}

// respond is helper to encode responses from the Server.
func (s *server) respond(w http.ResponseWriter, r *http.Request, data interface{}, status int) {
	w.WriteHeader(status)
	w.Header().Set("Content-Type", "application/json")

	if data != nil {
		err := json.NewEncoder(w).Encode(data)
		if err != nil {
			s.respondWithError(w, r, err, http.StatusInternalServerError)
			return
		}
	}
}

// respondWithError returns a standardized error message that is encoded by the *server.Respond function.
func (s *server) respondWithError(w http.ResponseWriter, r *http.Request, err error, status int) {
	type response struct {
		Status  string `json:"status"`
		Code    int    `json:"code"`
		Message string `json:"message"`
	}

	resp := response{
		Status:  http.StatusText(status),
		Code:    status,
		Message: err.Error(),
	}
	s.respond(w, r, resp, status)
}

// recoverer is a middleware that recovers from panics, logs the panic (and a
// backtrace), and returns a HTTP 500 (Internal Server Error) status if
// possible. Recoverer prints a request ID if one is provided.
func (s *server) recoverer(next http.Handler) http.Handler {
	fn := func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rvr := recover(); rvr != nil {
				errText := http.StatusText(http.StatusInternalServerError)
				requestID := xid.New()

				log.WithLevel(zerolog.PanicLevel).
					Str("req_id", requestID.String()).
					Str("method", r.Method).
					Str("url", r.URL.String()).
					Int("status", http.StatusInternalServerError).
					Dict("params", middleware.LogParamsAsDict(r.URL.Query())).
					Msg(fmt.Sprintf("Recover from Panic: %s; \n %s", rvr, debug.Stack()))

				err := fmt.Errorf("%s; error logged with request_id: %s", errText, requestID)
				s.respondWithError(w, r, err, http.StatusInternalServerError)
			}
		}()

		next.ServeHTTP(w, r)
	}

	return http.HandlerFunc(fn)
}