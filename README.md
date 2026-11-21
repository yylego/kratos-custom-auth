<!-- TEMPLATE (EN) BEGIN: BADGES -->

[![GitHub Workflow Status (branch)](https://img.shields.io/github/actions/workflow/status/yylego/kratos-custom-auth/release.yml?branch=main&label=BUILD)](https://github.com/yylego/kratos-custom-auth/actions/workflows/release.yml?query=branch%3Amain)
[![GoDoc](https://pkg.go.dev/badge/github.com/yylego/kratos-custom-auth)](https://pkg.go.dev/github.com/yylego/kratos-custom-auth)
[![Coverage Status](https://img.shields.io/coveralls/github/yylego/kratos-custom-auth/main.svg)](https://coveralls.io/github/yylego/kratos-custom-auth?branch=main)
[![Supported Go Versions](https://img.shields.io/badge/Go-1.26%2B-lightgrey.svg)](https://go.dev/)
[![GitHub Release](https://img.shields.io/github/release/yylego/kratos-custom-auth.svg)](https://github.com/yylego/kratos-custom-auth/releases)
[![Go Report Card](https://goreportcard.com/badge/github.com/yylego/kratos-custom-auth)](https://goreportcard.com/report/github.com/yylego/kratos-custom-auth)

<!-- TEMPLATE (EN) CLOSE: BADGES -->

# kratos-custom-auth

Custom token authentication middleware with self-defined validation functions.

---

<!-- TEMPLATE (EN) BEGIN: LANGUAGE NAVIGATION -->

## CHINESE README

[中文说明](README.zh.md)

<!-- TEMPLATE (EN) CLOSE: LANGUAGE NAVIGATION -->

## Main Features

- 🎯 **Custom Validation**: Authenticate with self-defined token check functions
- 🔌 **Context Injection**: Auto-inject authenticated data into request context
- 🛡️ **Route Scope Filtering**: Selective authentication based on route scope
- 🪵 **Debug Logs**: Each route match and auth outcome goes to the logger at debug level, and the logger level decides if it shows
- ⚙️ **Configurable Field**: Custom request field name, default is `Authorization`

## Installation

```bash
go get github.com/yylego/kratos-custom-auth
```

## Usage

```go
import (
    "context"
    "log/slog"

    "github.com/go-kratos/kratos/v3/errors"
    "github.com/go-kratos/kratos/v3/transport/http"
    "github.com/yylego/kratos-auth/authkratos"
    "github.com/yylego/kratos-custom-auth/customkratosauth"
)

type ctxAccount struct{}

func newHTTPServer(logger *slog.Logger) *http.Server {
    // Routes that need auth, written as the operation names that protoc-gen-go-http generates
    routeScope := authkratos.NewInclude(
        "/api.Student/CreateStudent",
        "/api.Student/UpdateStudent",
    )

    // Check the token and put the account into the context; a failed check returns the error the client gets
    checkToken := func(ctx context.Context, token string) (context.Context, *errors.Error) {
        account, ok := lookupAccount(token) // the token store you keep
        if !ok {
            return ctx, errors.Unauthorized("TOKEN_MISMATCH", "token mismatch")
        }
        return context.WithValue(ctx, ctxAccount{}, account), nil
    }

    cfg := customkratosauth.NewConfig(routeScope, checkToken).
        WithFieldName("AdminToken") // read the token from this request field, default is Authorization
    return http.NewServer(http.Middleware(customkratosauth.NewMiddleware(cfg, logger)))
}

// In the service code, read the account the check put in
func accountFrom(ctx context.Context) string {
    account, _ := ctx.Value(ctxAccount{}).(string)
    return account
}
```

Requests to routes in the scope get `401 UNAUTHORIZED` when the token is missing, and the error from `checkToken` when the check fails. Routes outside the scope pass through. To respond with a custom reason when the token is missing, use `WithMissingErk(func(ctx context.Context) *errors.Error { return errors.Unauthorized("TOKEN_MISMATCH", "please sign in") })`, so one branch in the client handles both cases; write a log inside this function when one is wanted.

---

<!-- TEMPLATE (EN) BEGIN: STANDARD PROJECT FOOTER -->
<!-- VERSION 2025-11-25 03:52:28.131064 +0000 UTC -->

## 📄 License

MIT License - see [LICENSE](LICENSE).

---

## 💬 Contact & Feedback

Contributions are welcome! Report bugs, suggest features, and contribute code:

- 🐛 **Mistake reports?** Open an issue on GitHub with reproduction steps
- 💡 **Fresh ideas?** Create an issue to discuss
- 📖 **Documentation confusing?** Report it so we can improve
- 🚀 **Need new features?** Share the use cases to help us understand requirements
- ⚡ **Performance issue?** Help us optimize through reporting slow operations
- 🔧 **Configuration problem?** Ask questions about complex setups
- 📢 **Follow project progress?** Watch the repo to get new releases and features
- 🌟 **Success stories?** Share how this package improved the workflow
- 💬 **Feedback?** We welcome suggestions and comments

---

## 🔧 Development

New code contributions, follow this process:

1. **Fork**: Fork the repo on GitHub (using the webpage UI).
2. **Clone**: Clone the forked project (`git clone https://github.com/yourname/repo-name.git`).
3. **Navigate**: Navigate to the cloned project (`cd repo-name`)
4. **Branch**: Create a feature branch (`git checkout -b feature/xxx`).
5. **Code**: Implement the changes with comprehensive tests
6. **Testing**: (Golang project) Ensure tests pass (`go test ./...`) and follow Go code style conventions
7. **Documentation**: Update documentation to support client-facing changes
8. **Stage**: Stage changes (`git add .`)
9. **Commit**: Commit changes (`git commit -m "Add feature xxx"`) ensuring backward compatible code
10. **Push**: Push to the branch (`git push origin feature/xxx`).
11. **PR**: Open a merge request on GitHub (on the GitHub webpage) with detailed description.

Please ensure tests pass and include relevant documentation updates.

---

## 🌟 Support

Welcome to contribute to this project via submitting merge requests and reporting issues.

**Project Support:**

- ⭐ **Give GitHub stars** if this project helps you
- 🤝 **Share with teammates** and (golang) programming friends
- 📝 **Write tech blogs** about development tools and workflows - we provide content writing support
- 🌟 **Join the ecosystem** - committed to supporting open source and the (golang) development scene

**Have Fun Coding with this package!** 🎉🎉🎉

<!-- TEMPLATE (EN) CLOSE: STANDARD PROJECT FOOTER -->
