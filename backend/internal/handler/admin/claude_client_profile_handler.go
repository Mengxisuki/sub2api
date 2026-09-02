package admin

import (
	"github.com/Wei-Shaw/sub2api/internal/pkg/claude"
	"github.com/Wei-Shaw/sub2api/internal/pkg/response"
	"github.com/gin-gonic/gin"
)

type ClaudeClientProfileHandler struct{}

func NewClaudeClientProfileHandler() *ClaudeClientProfileHandler {
	return &ClaudeClientProfileHandler{}
}

func (h *ClaudeClientProfileHandler) List(c *gin.Context) {
	response.Success(c, claude.ListClientProfiles())
}
