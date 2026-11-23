package httpapi

import (
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v2"
)

func TestInstanceMarketHTTPAuthAndStrictBody(t *testing.T) {
	for _, action := range []string{"quote", "list", "buy", "cancel", "read"} {
		for _, authenticated := range []bool{false, true} {
			for _, body := range []string{"", "{", `{"unexpected":true}`, `{} {}`, strings.Repeat(" ", 32769)} {
				t.Run(action+"/"+body[:min(len(body), 20)], func(t *testing.T) {
					s := &Server{App: fiber.New()}
					handlers := map[string]fiber.Handler{"quote": s.handleInstanceMarketQuote, "list": s.handleInstanceMarketList, "buy": s.handleInstanceMarketBuy, "cancel": s.handleInstanceMarketCancel, "read": s.handleInstanceMarketRead}
					s.Post("/test", func(c *fiber.Ctx) error {
						if authenticated {
							c.Locals("authUserID", int64(42))
						}
						return handlers[action](c)
					})
					request := httptest.NewRequest("POST", "/test?offset=invalid", strings.NewReader(body))
					request.Header.Set("Content-Type", "application/json")
					response, err := s.Test(request)
					if err != nil {
						t.Fatal(err)
					}
					defer response.Body.Close()
					want := 401
					if authenticated {
						want = 400
					}
					if response.StatusCode != want {
						t.Fatalf("status=%d want=%d", response.StatusCode, want)
					}
				})
			}
		}
	}
}
