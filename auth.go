package ginshow

import (
	"crypto/subtle"
	"net/http"
	"strings"

	"github.com/gin-gonic/gin"
)

func dashboardLoginPath(cfg Config) string {
	return cfg.DashboardPath + "/login"
}

func loginMethodNotAllowedHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.JSON(http.StatusMethodNotAllowed, gin.H{
			"error": "login requires POST",
			"hint":  "POST JSON {\"username\":\"...\",\"password\":\"...\"} or form fields username/password",
		})
	}
}

func loginHandler(auth *AuthConfig) gin.HandlerFunc {
	return func(c *gin.Context) {
		if auth == nil || auth.Username == "" {
			c.JSON(http.StatusNotFound, gin.H{"error": "auth disabled"})
			return
		}

		var req struct {
			Username string `json:"username" form:"username"`
			Password string `json:"password" form:"password"`
		}

		var err error
		if strings.HasPrefix(c.GetHeader("Content-Type"), "application/json") {
			err = c.ShouldBindJSON(&req)
		} else {
			err = c.ShouldBind(&req)
		}
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
			return
		}

		userOK := subtle.ConstantTimeCompare([]byte(req.Username), []byte(auth.Username)) == 1
		passOK := subtle.ConstantTimeCompare([]byte(req.Password), []byte(auth.Password)) == 1
		if !userOK || !passOK {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "invalid credentials"})
			return
		}

		c.JSON(http.StatusOK, gin.H{"ok": true})
	}
}
