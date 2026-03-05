package api

import (
	"bytes"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"io"
	stdlog "log"
	"net"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/klauspost/compress/zstd"

	"github.com/hteppl/remnawave-node-go/internal/api/controller"
	"github.com/hteppl/remnawave-node-go/internal/api/middleware"
	"github.com/hteppl/remnawave-node-go/internal/config"
	apperrors "github.com/hteppl/remnawave-node-go/internal/errors"
	"github.com/hteppl/remnawave-node-go/internal/logger"
	"github.com/hteppl/remnawave-node-go/internal/xray"
)

type Server struct {
	config             *config.Config
	logger             *logger.Logger
	core               *xray.Core
	configManager      *xray.ConfigManager
	xrayController     *controller.XrayController
	handlerController  *controller.HandlerController
	statsController    *controller.StatsController
	visionController   *controller.VisionController
	internalController *controller.InternalController
	mainServer         *http.Server
	internalServer     *http.Server
	mainRouter         *gin.Engine
	internalRouter     *gin.Engine
}

func NewServer(cfg *config.Config, log *logger.Logger, core *xray.Core, configMgr *xray.ConfigManager) (*Server, error) {
	gin.SetMode(gin.ReleaseMode)

	s := &Server{
		config:        cfg,
		logger:        log,
		core:          core,
		configManager: configMgr,
	}

	s.xrayController = controller.NewXrayController(core, configMgr, log)
	s.handlerController = controller.NewHandlerController(core, configMgr, log)
	s.statsController = controller.NewStatsController(core, log)
	s.visionController = controller.NewVisionController(core, log)
	s.internalController = controller.NewInternalController(configMgr, log)
	s.mainRouter = s.setupMainRouter()
	s.internalRouter = s.setupInternalRouter()

	tlsConfig, err := s.buildTLSConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to build TLS config: %w", err)
	}

	s.mainServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.NodePort),
		Handler:      s.mainRouter,
		TLSConfig:    tlsConfig,
		TLSNextProto: make(map[string]func(*http.Server, *tls.Conn, http.Handler)),
		ErrorLog:     stdlog.New(&tlsErrorFilter{s.logger}, "", 0),
	}

	s.internalServer = &http.Server{
		Addr:    fmt.Sprintf("127.0.0.1:%d", cfg.InternalRestPort),
		Handler: s.internalRouter,
	}

	return s, nil
}

func (s *Server) buildTLSConfig() (*tls.Config, error) {
	cert, err := tls.X509KeyPair(
		[]byte(s.config.Payload.NodeCertPEM),
		[]byte(s.config.Payload.NodeKeyPEM),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to load server certificate: %w", err)
	}

	caCertPool := x509.NewCertPool()
	if !caCertPool.AppendCertsFromPEM([]byte(s.config.Payload.CACertPEM)) {
		return nil, fmt.Errorf("failed to parse CA certificate")
	}

	return &tls.Config{
		Certificates: []tls.Certificate{cert},
		ClientAuth:   tls.RequireAndVerifyClientCert,
		ClientCAs:    caCertPool,
		MinVersion:   tls.VersionTLS12,
	}, nil
}

func (s *Server) setupMainRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(s.loggingMiddleware())
	router.Use(s.zstdMiddleware())
	router.Use(middleware.JWTMiddleware(s.config.Payload.JWTPublicKey, s.logger))

	router.NoRoute(s.notFoundHandler())

	nodeGroup := router.Group("/node")
	{
		xrayGroup := nodeGroup.Group("/xray")
		s.xrayController.RegisterRoutes(xrayGroup)

		handlerGroup := nodeGroup.Group("/handler")
		s.handlerController.RegisterRoutes(handlerGroup)

		statsGroup := nodeGroup.Group("/stats")
		s.statsController.RegisterRoutes(statsGroup)
	}

	return router
}

func (s *Server) setupInternalRouter() *gin.Engine {
	router := gin.New()
	router.Use(gin.Recovery())
	router.Use(s.loggingMiddleware())
	router.Use(PortGuardMiddleware(s.config.InternalRestPort))

	router.NoRoute(func(c *gin.Context) {
		c.String(404, "Cannot %s %s", c.Request.Method, c.Request.URL.Path)
	})

	internalGroup := router.Group("/internal")
	{
		s.internalController.RegisterRoutes(internalGroup)
	}

	visionGroup := router.Group("/vision")
	{
		s.visionController.RegisterRoutes(visionGroup)
	}

	return router
}

func (s *Server) MainRouter() *gin.Engine {
	return s.mainRouter
}

func (s *Server) InternalRouter() *gin.Engine {
	return s.internalRouter
}

func (s *Server) loggingMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
	}
}

func (s *Server) zstdMiddleware() gin.HandlerFunc {
	decoder, _ := zstd.NewReader(nil)

	return func(c *gin.Context) {
		if c.GetHeader("Content-Encoding") == "zstd" {
			body, err := io.ReadAll(c.Request.Body)
			if err != nil {
				c.AbortWithStatus(400)
				return
			}
			decompressed, err := decoder.DecodeAll(body, nil)
			if err != nil {
				c.AbortWithStatus(400)
				return
			}
			c.Request.Body = io.NopCloser(bytes.NewReader(decompressed))
			c.Request.Header.Del("Content-Encoding")
			c.Request.ContentLength = int64(len(decompressed))
		}
		c.Next()
	}
}

func (s *Server) notFoundHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		destroySocket(c)
	}
}

func (s *Server) Start() error {
	errCh := make(chan error, 2)

	go func() {
		s.logger.Info(fmt.Sprintf("Starting main HTTPS server on :%d", s.config.NodePort))
		if err := s.mainServer.ListenAndServeTLS("", ""); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("main server error: %w", err)
		}
	}()

	go func() {
		s.logger.Info(fmt.Sprintf("Starting internal HTTP server on 127.0.0.1:%d", s.config.InternalRestPort))
		if err := s.internalServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errCh <- fmt.Errorf("internal server error: %w", err)
		}
	}()

	select {
	case err := <-errCh:
		return err
	default:
		return nil
	}
}

func (s *Server) Stop() error {
	if err := s.mainServer.Close(); err != nil {
		return err
	}
	if err := s.internalServer.Close(); err != nil {
		return err
	}
	return nil
}

func destroySocket(c *gin.Context) {
	defer func() {
		_ = recover()
		c.Abort()
	}()

	hijacker, ok := c.Writer.(http.Hijacker)
	if !ok {
		return
	}
	conn, _, err := hijacker.Hijack()
	if err != nil {
		return
	}
	conn.Close()
}

func PortGuardMiddleware(expectedPort int) gin.HandlerFunc {
	return func(c *gin.Context) {
		localAddr := c.Request.Context().Value(http.LocalAddrContextKey)
		if localAddr == nil {
			destroySocket(c)
			return
		}

		tcpAddr, ok := localAddr.(*net.TCPAddr)
		if !ok {
			destroySocket(c)
			return
		}

		if tcpAddr.Port != expectedPort || tcpAddr.IP.String() != "127.0.0.1" {
			destroySocket(c)
			return
		}

		c.Next()
	}
}

type tlsErrorFilter struct {
	logger *logger.Logger
}

func (f *tlsErrorFilter) Write(p []byte) (n int, err error) {
	msg := strings.TrimSpace(string(p))
	if strings.Contains(msg, "TLS handshake error") {
		return len(p), nil
	}
	if f.logger != nil {
		f.logger.Error(msg)
	}
	return len(p), nil
}

func ErrorHandler(code string, c *gin.Context) {
	errDef, ok := apperrors.GetError(code)
	if !ok {
		errDef = apperrors.ERRORS[apperrors.CodeInternalServerError]
	}

	c.JSON(errDef.HTTPCode, NewErrorResponse(
		c.Request.URL.Path,
		errDef.Message,
		errDef.Code,
	))
}
