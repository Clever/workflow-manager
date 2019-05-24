package server

// Code auto-generated. Do not edit.

import (
	"log"
	"net/http"
	_ "net/http/pprof"
	"os"
	"path"
	"time"

	"github.com/Clever/go-process-metrics/metrics"
	"github.com/gorilla/mux"
	"github.com/kardianos/osext"
	opentracing "github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go"
	jaegercfg "github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-client-go/transport"
	"gopkg.in/Clever/kayvee-go.v6/logger"
	kvMiddleware "gopkg.in/Clever/kayvee-go.v6/middleware"
	"gopkg.in/tylerb/graceful.v1"
)

type contextKey struct{}

// Server defines a HTTP server that implements the Controller interface.
type Server struct {
	// Handler should generally not be changed. It exposed to make testing easier.
	Handler http.Handler
	addr    string
	l       logger.KayveeLogger
}

// Serve starts the server. It will return if an error occurs.
func (s *Server) Serve() error {

	go func() {
		metrics.Log("workflow-manager", 1*time.Minute)
	}()

	go func() {
		// This should never return. Listen on the pprof port
		log.Printf("PProf server crashed: %s", http.ListenAndServe("localhost:6060", nil))
	}()

	dir, err := osext.ExecutableFolder()
	if err != nil {
		log.Fatal(err)
	}
	if err := logger.SetGlobalRouting(path.Join(dir, "kvconfig.yml")); err != nil {
		s.l.Info("please provide a kvconfig.yml file to enable app log routing")
	}

	if tracingToken := os.Getenv("TRACING_ACCESS_TOKEN"); tracingToken != "" {
		ingestUrl := os.Getenv("TRACING_INGEST_URL")
		if ingestUrl == "" {
			ingestUrl = "https://ingest.signalfx.com/v1/trace"
		}

		// Create a Jaeger HTTP Thrift transport
		transport := transport.NewHTTPTransport(ingestUrl,
			transport.HTTPBasicAuth("auth", tracingToken))

		// Add rate limited sampling. We will only sample [Param] requests per second
		// and [MaxOperations] different endpoints. Any endpoint above the [MaxOperations]
		// limit will be probabilistically sampled.
		cfgSampler := &jaegercfg.SamplerConfig{
			Type:          jaeger.SamplerTypeRateLimiting,
			Param:         5,
			MaxOperations: 100,
		}
		cfgTags := []opentracing.Tag{
			opentracing.Tag{Key: "app_name", Value: os.Getenv("_APP_NAME")},
			opentracing.Tag{Key: "build_id", Value: os.Getenv("_BUILD_ID")},
			opentracing.Tag{Key: "deploy_env", Value: os.Getenv("_DEPLOY_ENV")},
			opentracing.Tag{Key: "team_owner", Value: os.Getenv("_TEAM_OWNER")},
		}

		cfg := &jaegercfg.Configuration{
			ServiceName: "workflow-manager",
			Sampler:     cfgSampler,
			Tags:        cfgTags,
		}

		signalfxTracer, closer, err := cfg.NewTracer(jaegercfg.Reporter(jaeger.NewRemoteReporter(transport)))
		if err != nil {
			log.Fatal("Could not initialize jaeger tracer: ", err.Error())
		}
		defer closer.Close()

		opentracing.SetGlobalTracer(signalfxTracer)
	} else {
		s.l.Error("please set TRACING_ACCESS_TOKEN to enable tracing")
	}

	s.l.Counter("server-started")

	// Give the sever 30 seconds to shut down
	return graceful.RunWithErr(s.addr, 30*time.Second, s.Handler)
}

type handler struct {
	Controller
}

func withMiddleware(serviceName string, router http.Handler, m []func(http.Handler) http.Handler) http.Handler {
	handler := router

	// Wrap the middleware in the opposite order specified so that when called then run
	// in the order specified
	for i := len(m) - 1; i >= 0; i-- {
		handler = m[i](handler)
	}
	handler = TracingMiddleware(handler)
	handler = PanicMiddleware(handler)
	// Logging middleware comes last, i.e. will be run first.
	// This makes it so that other middleware has access to the logger
	// that kvMiddleware injects into the request context.
	handler = kvMiddleware.New(handler, serviceName)
	return handler
}

// New returns a Server that implements the Controller interface. It will start when "Serve" is called.
func New(c Controller, addr string) *Server {
	return NewWithMiddleware(c, addr, []func(http.Handler) http.Handler{})
}

// NewWithMiddleware returns a Server that implemenets the Controller interface. It runs the
// middleware after the built-in middleware (e.g. logging), but before the controller methods.
// The middleware is executed in the order specified. The server will start when "Serve" is called.
func NewWithMiddleware(c Controller, addr string, m []func(http.Handler) http.Handler) *Server {
	router := mux.NewRouter()
	h := handler{Controller: c}

	l := logger.New("workflow-manager")

	router.Methods("GET").Path("/_health").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "healthCheck")
		h.HealthCheckHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "healthCheck")
		r = r.WithContext(ctx)
	})

	router.Methods("POST").Path("/state-resources").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "postStateResource")
		h.PostStateResourceHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "postStateResource")
		r = r.WithContext(ctx)
	})

	router.Methods("DELETE").Path("/state-resources/{namespace}/{name}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "deleteStateResource")
		h.DeleteStateResourceHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "deleteStateResource")
		r = r.WithContext(ctx)
	})

	router.Methods("GET").Path("/state-resources/{namespace}/{name}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "getStateResource")
		h.GetStateResourceHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "getStateResource")
		r = r.WithContext(ctx)
	})

	router.Methods("PUT").Path("/state-resources/{namespace}/{name}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "putStateResource")
		h.PutStateResourceHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "putStateResource")
		r = r.WithContext(ctx)
	})

	router.Methods("GET").Path("/workflow-definitions").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "getWorkflowDefinitions")
		h.GetWorkflowDefinitionsHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "getWorkflowDefinitions")
		r = r.WithContext(ctx)
	})

	router.Methods("POST").Path("/workflow-definitions").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "newWorkflowDefinition")
		h.NewWorkflowDefinitionHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "newWorkflowDefinition")
		r = r.WithContext(ctx)
	})

	router.Methods("GET").Path("/workflow-definitions/{name}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "getWorkflowDefinitionVersionsByName")
		h.GetWorkflowDefinitionVersionsByNameHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "getWorkflowDefinitionVersionsByName")
		r = r.WithContext(ctx)
	})

	router.Methods("PUT").Path("/workflow-definitions/{name}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "updateWorkflowDefinition")
		h.UpdateWorkflowDefinitionHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "updateWorkflowDefinition")
		r = r.WithContext(ctx)
	})

	router.Methods("GET").Path("/workflow-definitions/{name}/{version}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "getWorkflowDefinitionByNameAndVersion")
		h.GetWorkflowDefinitionByNameAndVersionHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "getWorkflowDefinitionByNameAndVersion")
		r = r.WithContext(ctx)
	})

	router.Methods("GET").Path("/workflows").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "getWorkflows")
		h.GetWorkflowsHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "getWorkflows")
		r = r.WithContext(ctx)
	})

	router.Methods("POST").Path("/workflows").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "startWorkflow")
		h.StartWorkflowHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "startWorkflow")
		r = r.WithContext(ctx)
	})

	router.Methods("DELETE").Path("/workflows/{workflowID}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "CancelWorkflow")
		h.CancelWorkflowHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "CancelWorkflow")
		r = r.WithContext(ctx)
	})

	router.Methods("GET").Path("/workflows/{workflowID}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "getWorkflowByID")
		h.GetWorkflowByIDHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "getWorkflowByID")
		r = r.WithContext(ctx)
	})

	router.Methods("POST").Path("/workflows/{workflowID}").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "resumeWorkflowByID")
		h.ResumeWorkflowByIDHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "resumeWorkflowByID")
		r = r.WithContext(ctx)
	})

	router.Methods("POST").Path("/workflows/{workflowID}/resolved").HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		logger.FromContext(r.Context()).AddContext("op", "resolveWorkflowByID")
		h.ResolveWorkflowByIDHandler(r.Context(), w, r)
		ctx := WithTracingOpName(r.Context(), "resolveWorkflowByID")
		r = r.WithContext(ctx)
	})

	handler := withMiddleware("workflow-manager", router, m)
	return &Server{Handler: handler, addr: addr, l: l}
}
