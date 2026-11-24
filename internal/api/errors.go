package api

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"walrus/internal/apperr"
)

func respondError(c *gin.Context, status int, err error) {
	fallback := apperr.CodeInternalError
	if status == http.StatusBadRequest {
		fallback = apperr.CodeInvalidRequest
	} else if status == http.StatusNotFound {
		fallback = apperr.CodeNotFound
	} else if status == http.StatusForbidden {
		fallback = apperr.CodeForbidden
	} else if status == http.StatusUnauthorized {
		fallback = apperr.CodeUnauthorized
	}
	c.JSON(status, apperr.Body(err, fallback))
}

func respondCodeError(c *gin.Context, status int, code, message string, params ...map[string]any) {
	var p map[string]any
	if len(params) > 0 {
		p = params[0]
	}
	c.JSON(status, apperr.CodeBody(code, message, p))
}

func respondBindError(c *gin.Context, err error) {
	c.JSON(http.StatusBadRequest, apperr.CodeBody(apperr.CodeInvalidRequest, err.Error(), nil))
}
