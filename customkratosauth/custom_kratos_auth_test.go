package customkratosauth

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	nethttp "net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-kratos/kratos/v3/errors"
	"github.com/go-kratos/kratos/v3/transport/http"
	"github.com/stretchr/testify/require"
	"github.com/yylego/kratos-auth/authkratos"
)

type ctxAccount struct{}

// checkToken accepts the token "secret-token" and puts the account name into the context
// checkToken 只认 "secret-token"，并把账号名放进 context
func checkToken(ctx context.Context, token string) (context.Context, *errors.Error) {
	if token != "secret-token" {
		return ctx, errors.Unauthorized("TOKEN_MISMATCH", "token mismatch")
	}
	return context.WithValue(ctx, ctxAccount{}, "alice"), nil
}

// newTestServer starts a Kratos HTTP server with the auth middleware and two routes:
// /api.Demo/Profile needs auth and echoes the account name, /api.Demo/Health is outside the scope
//
// newTestServer 启动带认证中间件的 Kratos HTTP 服务，挂两个路由：
// /api.Demo/Profile 要认证并回显账号名，/api.Demo/Health 不在认证范围内
func newTestServer(t *testing.T, cfg *Config, logger *slog.Logger) *httptest.Server {
	srv := http.NewServer(http.Middleware(NewMiddleware(cfg, logger)))
	route := srv.Route("/")
	route.GET("/profile", func(ctx http.Context) error {
		http.SetOperation(ctx, "/api.Demo/Profile")
		handle := ctx.Middleware(func(ctx context.Context, req any) (any, error) {
			account, _ := ctx.Value(ctxAccount{}).(string)
			return map[string]string{"account": account}, nil
		})
		res, err := handle(ctx, nil)
		if err != nil {
			return err
		}
		return ctx.Result(200, res)
	})
	route.GET("/health", func(ctx http.Context) error {
		http.SetOperation(ctx, "/api.Demo/Health")
		handle := ctx.Middleware(func(ctx context.Context, req any) (any, error) {
			return map[string]string{"status": "ok"}, nil
		})
		res, err := handle(ctx, nil)
		if err != nil {
			return err
		}
		return ctx.Result(200, res)
	})
	server := httptest.NewServer(srv)
	t.Cleanup(server.Close)
	return server
}

// sendRequest sends a GET request with the given request field and returns the status code and the body
// sendRequest 发一个带指定请求头的 GET 请求，返回状态码和响应体
func sendRequest(t *testing.T, address string, fieldName string, token string) (int, string) {
	request, err := nethttp.NewRequest(nethttp.MethodGet, address, nil)
	require.NoError(t, err)
	if token != "" {
		request.Header.Set(fieldName, token)
	}
	response, err := nethttp.DefaultClient.Do(request)
	require.NoError(t, err)
	defer func() {
		require.NoError(t, response.Body.Close())
	}()
	body, err := io.ReadAll(response.Body)
	require.NoError(t, err)
	return response.StatusCode, string(body)
}

func TestNewConfig(t *testing.T) {
	cfg := NewConfig(authkratos.NewInclude("/api.Demo/Profile"), checkToken)
	require.Equal(t, "Authorization", cfg.FieldName())
}

func TestConfig_WithFieldName(t *testing.T) {
	cfg := NewConfig(authkratos.NewInclude("/api.Demo/Profile"), checkToken).WithFieldName("X-Token")
	server := newTestServer(t, cfg, slog.Default())

	code, body := sendRequest(t, server.URL+"/profile", "X-Token", "secret-token")
	require.Equal(t, 200, code)
	require.Equal(t, `{"account":"alice"}`, body)

	// The default field stops counting once a custom one is set
	// 设了自定义字段后，默认的 Authorization 不再生效
	code, _ = sendRequest(t, server.URL+"/profile", "Authorization", "secret-token")
	require.Equal(t, 401, code)
}

func TestConfig_WithMissingErk(t *testing.T) {
	cfg := NewConfig(authkratos.NewInclude("/api.Demo/Profile"), checkToken).
		WithMissingErk(func(ctx context.Context) *errors.Error {
			return errors.Unauthorized("TOKEN_MISMATCH", "please sign in")
		})
	server := newTestServer(t, cfg, slog.Default())

	code, body := sendRequest(t, server.URL+"/profile", "Authorization", "")
	require.Equal(t, 401, code)
	require.Equal(t, `{"code":401,"reason":"TOKEN_MISMATCH","message":"please sign in"}`, body)
}

func TestConfig_FieldName(t *testing.T) {
	cfg := NewConfig(authkratos.NewInclude("/api.Demo/Profile"), checkToken)
	require.Equal(t, "Authorization", cfg.FieldName())
	require.Equal(t, "X-Token", cfg.WithFieldName("X-Token").FieldName())
}

// TestNewMiddleware_DebugLogs checks the debug logs show at debug level and are hidden at info level
// TestNewMiddleware_DebugLogs 检查调试日志在 debug 级别显示、在 info 级别不显示
func TestNewMiddleware_DebugLogs(t *testing.T) {
	collectLogs := func(level slog.Level) string {
		var output bytes.Buffer
		logger := slog.New(slog.NewTextHandler(&output, &slog.HandlerOptions{
			Level: level,
			ReplaceAttr: func(groups []string, attr slog.Attr) slog.Attr {
				if attr.Key == slog.TimeKey {
					return slog.Attr{} // Drop the time so the output stays the same on each run // 去掉时间，每次输出才一样
				}
				return attr
			},
		}))
		server := newTestServer(t, NewConfig(authkratos.NewInclude("/api.Demo/Profile"), checkToken), logger)
		// One request of each outcome: no token, a wrong token, the right token
		// 三种结果各发一个请求：没带令牌、令牌不对、令牌正确
		code, _ := sendRequest(t, server.URL+"/profile", "Authorization", "")
		require.Equal(t, 401, code)
		code, _ = sendRequest(t, server.URL+"/profile", "Authorization", "wrong-token")
		require.Equal(t, 401, code)
		code, _ = sendRequest(t, server.URL+"/profile", "Authorization", "secret-token")
		require.Equal(t, 200, code)
		return output.String()
	}

	require.Equal(t, `level=INFO msg="custom-kratos-auth: new middleware" field-name=Authorization side=INCLUDE operations=1
`, collectLogs(slog.LevelInfo))

	require.Equal(t, `level=INFO msg="custom-kratos-auth: new middleware" field-name=Authorization side=INCLUDE operations=1
level=DEBUG msg="custom-kratos-auth: new middleware route-scope" route-scope="&{Side:INCLUDE OperationSet:map[/api.Demo/Profile:true]}"
level=DEBUG msg="custom-kratos-auth: route match" operation=/api.Demo/Profile side=INCLUDE match=true
level=DEBUG msg="custom-kratos-auth: request rejected, auth-token is missing"
level=DEBUG msg="custom-kratos-auth: route match" operation=/api.Demo/Profile side=INCLUDE match=true
level=DEBUG msg="custom-kratos-auth: request rejected, auth-token mismatch" reason="error: code = 401 reason = TOKEN_MISMATCH message = token mismatch metadata = map[] cause = <nil>"
level=DEBUG msg="custom-kratos-auth: route match" operation=/api.Demo/Profile side=INCLUDE match=true
level=DEBUG msg="custom-kratos-auth: request passed"
`, collectLogs(slog.LevelDebug))
}

func TestNewMiddleware(t *testing.T) {
	cfg := NewConfig(authkratos.NewInclude("/api.Demo/Profile"), checkToken)
	server := newTestServer(t, cfg, slog.Default())

	t.Run("valid-token-passes-with-the-account", func(t *testing.T) {
		code, body := sendRequest(t, server.URL+"/profile", "Authorization", "secret-token")
		require.Equal(t, 200, code)
		require.Equal(t, `{"account":"alice"}`, body)
	})

	t.Run("missing-token-is-unauthorized", func(t *testing.T) {
		code, body := sendRequest(t, server.URL+"/profile", "Authorization", "")
		require.Equal(t, 401, code)
		require.Equal(t, `{"code":401,"reason":"UNAUTHORIZED","message":"custom-kratos-auth: auth-token is missing"}`, body)
	})

	t.Run("wrong-token-returns-the-check-error", func(t *testing.T) {
		code, body := sendRequest(t, server.URL+"/profile", "Authorization", "wrong-token")
		require.Equal(t, 401, code)
		require.Equal(t, `{"code":401,"reason":"TOKEN_MISMATCH","message":"token mismatch"}`, body)
	})

	t.Run("route-outside-the-scope-needs-no-token", func(t *testing.T) {
		code, body := sendRequest(t, server.URL+"/health", "Authorization", "")
		require.Equal(t, 200, code)
		require.Equal(t, `{"status":"ok"}`, body)
	})
}
