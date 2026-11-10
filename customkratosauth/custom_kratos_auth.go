// Package customkratosauth: Custom token authentication middleware with user-defined validation
// Provides flexible auth middleware with custom token check functions and context injection
// Supports route scope filtering and configurable request field names
// Injects authenticated data into context on success
//
// customkratosauth: 自定义令牌认证中间件，支持用户定义的验证逻辑
// 提供灵活的认证中间件，支持用户定义的令牌检查函数和上下文注入
// 支持路由范围过滤和可配置的请求头字段名
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
// Combines route scope and token validation function
// Note: Avoid non-standard names in production (Nginx drops request fields with underscores unless configured)
//
// Config 保存自定义认证中间件的配置
// 组合路由范围和令牌验证函数
// 注意：生产环境避免非标准字段名（Nginx 默认丢弃带下划线的请求头，除非配置）
type Config struct {
	routeScope *authkratos.RouteScope  // Route scope which auth applies to // 认证应用的路由范围
	checkToken CheckTokenAndSetCtxFunc // Custom token validation function // 自定义令牌验证函数
	fieldName  string                  // Request field name extracting auth token // 提取认证令牌的请求头字段名
}

// NewConfig creates a new custom auth config with route scope and token check function
// Defaults to the Authorization field
//
// NewConfig 创建新的自定义认证配置，需要路由范围和令牌检查函数
// 默认使用 Authorization 请求头
func NewConfig(routeScope *authkratos.RouteScope, checkToken CheckTokenAndSetCtxFunc) *Config {
	return &Config{
		routeScope: routeScope,
		checkToken: checkToken,
		fieldName:  "Authorization",
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

// NewMiddleware creates the auth middleware: routes inside the scope need a token that passes checkToken
// A missing token gets 401 UNAUTHORIZED, a failed check returns the error from checkToken, routes outside the scope pass through
// Each match and each auth outcome goes to applog at debug level, so the level of applog decides if it shows
//
// NewMiddleware 创建认证中间件：范围内的路由要带令牌，并通过 checkToken 校验
// 没带令牌返回 401 UNAUTHORIZED，校验失败返回 checkToken 给的错误，范围外的路由直接放行
// 每次匹配和每次认证结果都以 debug 级别写进 applog，显示与否由 applog 的日志级别决定
func NewMiddleware(cfg *Config, applog *slog.Logger) middleware.Middleware {
	applog.Info(
		"custom-kratos-auth: new middleware",
		"field-name", cfg.fieldName,
		"side", cfg.routeScope.Side,
		"operations", len(cfg.routeScope.OperationSet),
	)
	applog.Debug("custom-kratos-auth: new middleware route-scope", "route-scope", cfg.routeScope)
	return selector.Server(middlewareFunc(cfg, applog)).Match(matchFunc(cfg, applog)).Build()
}

func matchFunc(cfg *Config, applog *slog.Logger) selector.MatchFunc {
	return func(ctx context.Context, operation string) bool {
		match := cfg.routeScope.Match(operation)
		applog.Debug("custom-kratos-auth: route match", "operation", operation, "side", cfg.routeScope.Side, "match", match)
		return match
	}
}

func middlewareFunc(cfg *Config, applog *slog.Logger) middleware.Middleware {
	return func(handleFunc middleware.Handler) middleware.Handler {
		return func(ctx context.Context, req any) (any, error) {
			tsp, ok := transport.FromServerContext(ctx)
			if !ok {
				return nil, errors.Unauthorized("UNAUTHORIZED", "custom-kratos-auth: wrong context")
			}
			authToken := tsp.RequestHeader().Get(cfg.fieldName)
			if authToken == "" {
				applog.Debug("custom-kratos-auth: auth-token is missing")
				return nil, errors.Unauthorized("UNAUTHORIZED", "custom-kratos-auth: auth-token is missing")
			}
			ctx, erk := cfg.checkToken(ctx, authToken)
			if erk != nil {
				applog.Debug("custom-kratos-auth: auth-token mismatch", "reason", erk.Error())
				return nil, erk
			}
			return handleFunc(ctx, req)
		}
	}
}
