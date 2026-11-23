package httpapi

import "github.com/gofiber/fiber/v2"

const emailDomainBlockedMessageKey = "auth.error.emailDomainBlocked"

func (s *Server) isEmailDomainBlocked(email string) bool {
	return s.deps.EmailPolicy != nil && s.deps.EmailPolicy.IsBlocked(email)
}

func (s *Server) writeEmailDomainBlocked(c *fiber.Ctx) error {
	return s.writeAPIErrorWithMessageKey(
		c,
		fiber.StatusBadRequest,
		codeAuthEmailDomainBlocked,
		"Email domain is not supported",
		emailDomainBlockedMessageKey,
		"",
	)
}
