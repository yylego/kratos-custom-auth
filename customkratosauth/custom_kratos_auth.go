// Package customkratosauth: Custom token authentication middleware with user-defined validation
// Provides flexible auth middleware with custom token check functions and context injection
// Supports route scope filtering, span tracing callback, and configurable request field names
// Injects authenticated data into context on success
//
// customkratosauth: 自定义令牌认证中间件，支持用户定义的验证逻辑
// 提供灵活的认证中间件，支持用户定义的令牌检查函数和上下文注入
// 支持路由范围过滤、span 追踪回调和可配置的请求头字段名
// 认证成功时可以将用户信息注入到上下文中
package customkratosauth

import (
	"context"
	"log/slog"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/middleware"
	"github.com/go-kratos/kratos/v3/middleware/selector"
	"github.com/go-kratos/kratos/v3/transport"
	"github.com/yylego/kratos-auth/authkratos"
	"github.com/yylego/neatjson/neatjsons"
)

// CheckTokenAndSetCtxFunc validates auth token and injects account data into context
// Parameters: ctx - current request context, token - authentication token
// Returns: new context (with account data if present) and validation status
// On success, account data gets injected into context accessible to downstream handlers
//
// CheckTokenAndSetCtxFunc 验证认证令牌并将用户信息注入上下文
// 参数：ctx - 当前请求上下文，token - 认证令牌
// 返回：新的 context（可能包含用户信息）和错误
// 认证成功时可以将用户信息注入到返回的 context 中，供后续处理程序使用
type CheckTokenAndSetCtxFunc func(ctx context.Context, token string) (context.Context, *errors.Error)

// Config holds the custom auth middleware configuration
// Combines route scope, token validation function, and span tracing callback
// Note: Avoid non-standard names in production (Nginx drops request fields with underscores unless configured)
//
// Config 保存自定义认证中间件的配置
// 组合路由范围、令牌验证函数和 span 追踪回调
// 注意：生产环境避免非标准字段名（Nginx 默认丢弃带下划线的请求头，除非配置）
type Config struct {
	routeScope *authkratos.RouteScope       // Route scope which auth applies to // 认证应用的路由范围
	checkToken CheckTokenAndSetCtxFunc      // Custom token validation function // 自定义令牌验证函数
	fieldName  string                       // Request field name extracting auth token // 提取认证令牌的请求头字段名
	spanHooks  []authkratos.NewSpanHookFunc // Span hook factories, empty disables tracing // span 钩子工厂列表，为空时禁用追踪
	debugMode  bool                         // Debug mode switch // 调试模式开关
}

// NewConfig creates a new custom auth config with route scope and token check function
// Defaults to Authorization field and debug mode disabled
//
// NewConfig 创建新的自定义认证配置，需要路由范围和令牌检查函数
// 默认使用 Authorization 请求头，调试模式默认关闭
func NewConfig(routeScope *authkratos.RouteScope, checkToken CheckTokenAndSetCtxFunc) *Config {
	return &Config{
		routeScope: routeScope,
		checkToken: checkToken,
		fieldName:  "Authorization",
		spanHooks:  nil,
		debugMode:  false,
	}
}

// WithFieldName sets request field name used in authentication
// Avoid non-standard names in configuration
// Nginx ignores names with underscores unless underscores_in_headers is on
// Recommend not using names with extra punctuation in development
//
// WithFieldName 设置请求头中用于认证的字段名
// 注意配置时不要配置非标准的字段名
// Nginx 默认忽略带有下划线的 headers 信息，除非配置 underscores_in_headers on
// 因此在开发中建议不要配置含特殊字符的字段名
func (c *Config) WithFieldName(fieldName string) *Config {
	c.fieldName = fieldName
	return c
}

// GetFieldName gets request field name used in authentication
//
// GetFieldName 获取请求头中用于认证的字段名
func (c *Config) GetFieldName() string {
	return c.fieldName
}

func (c *Config) WithDebugMode(debugMode bool) *Config {
	c.debugMode = debugMode
	return c
}

// WithNewSpanHook appends a span hook factory to the configuration
// When span hooks are present, tracing hooks are invoked for match and middleware operations
//
// WithNewSpanHook 追加一个 span 钩子工厂到配置中
// 当存在 span 钩子时，match 和 middleware 操作会调用追踪钩子
func (c *Config) WithNewSpanHook(fn authkratos.NewSpanHookFunc) *Config {
	c.spanHooks = append(c.spanHooks, fn)
	return c
}

func NewMiddleware(cfg *Config, applog *slog.Logger) middleware.Middleware {
	applog.Info(
		"custom-kratos-auth: new middleware",
		"field-name", cfg.fieldName,
		"side", cfg.routeScope.Side,
		"operations", len(cfg.routeScope.OperationSet),
		"debug-mode", authkratos.BooleanToNum(cfg.debugMode),
	)
	if cfg.debugMode {
		applog.Debug("custom-kratos-auth: new middleware route-scope", "field-name", cfg.fieldName, "route-scope", neatjsons.S(cfg.routeScope))
	}
	return selector.Server(middlewareFunc(cfg, applog)).Match(matchFunc(cfg, applog)).Build()
}

func matchFunc(cfg *Config, applog *slog.Logger) selector.MatchFunc {
	return func(ctx context.Context, operation string) bool {
		defer authkratos.RunSpanHooks(ctx, cfg.spanHooks, "custom-kratos-auth-match")()

		match := cfg.routeScope.Match(operation)
		if cfg.debugMode {
			if match {
				applog.Debug("custom-kratos-auth: match next -> check auth", "operation", operation, "side", cfg.routeScope.Side, "match", authkratos.BooleanToNum(match))
			} else {
				applog.Debug("custom-kratos-auth: match skip -- check auth", "operation", operation, "side", cfg.routeScope.Side, "match", authkratos.BooleanToNum(match))
			}
		}
		return match
	}
}

func middlewareFunc(cfg *Config, applog *slog.Logger) middleware.Middleware {
	return func(handleFunc middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req interface{}) (interface{}, error) {
			if tsp, ok := transport.FromServerContext(ctx); ok {
				defer authkratos.RunSpanHooks(ctx, cfg.spanHooks, "custom-kratos-auth")()

				authToken := tsp.RequestHeader().Get(cfg.fieldName)
				if authToken == "" {
					if cfg.debugMode {
						applog.Debug("custom-kratos-auth: auth-token is missing")
					}
					return nil, errors.Unauthorized("UNAUTHORIZED", "custom-kratos-auth: auth-token is missing")
				}
				ctx, erk := cfg.checkToken(ctx, authToken)
				if erk != nil {
					if cfg.debugMode {
						applog.Debug("custom-kratos-auth: auth-token mismatch", "reason", erk.Error())
					}
					return nil, erk
				}
				return handleFunc(ctx, req)
			}
			return nil, errors.Unauthorized("UNAUTHORIZED", "custom-kratos-auth: wrong context")
		}
	}
}
