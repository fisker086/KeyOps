package mcp

import (
	"encoding/json"
	"io"
	"net/http"

	"github.com/fisker086/keyops/internal/model"
	"github.com/gin-gonic/gin"
)

type Handler struct {
	server *Server
}

func NewHandler(server *Server) *Handler {
	return &Handler{server: server}
}

func (h *Handler) ListTools(c *gin.Context) {
	tools := h.server.Registry().List()
	type toolInfo struct {
		Name        string `json:"name"`
		Description string `json:"description"`
	}
	items := make([]toolInfo, 0, len(tools))
	for _, t := range tools {
		items = append(items, toolInfo{Name: t.Name, Description: t.Description})
	}
	c.JSON(http.StatusOK, model.Success(items))
}

func (h *Handler) HandleMCPGet(c *gin.Context) {
	tools := h.server.Registry().List()
	names := make([]string, 0, len(tools))
	for _, t := range tools {
		names = append(names, t.Name)
	}
	c.JSON(http.StatusOK, gin.H{
		"server":    "k8s-mcp",
		"version":   "0.1.0",
		"transport": "streamable-http",
		"tools":     names,
	})
}

func (h *Handler) HandleMCP(c *gin.Context) {
	body, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(http.StatusBadRequest, model.Error(400, "failed to read request body"))
		return
	}

	if len(body) == 0 {
		c.JSON(http.StatusBadRequest, model.Error(400, "empty request body"))
		return
	}

	allowedTools, _ := c.Get("mcp_permissions")
	perms, _ := allowedTools.([]string)

	var rawMessages []json.RawMessage
	if body[0] == '[' {
		if err := json.Unmarshal(body, &rawMessages); err != nil {
			c.JSON(http.StatusBadRequest, model.Error(400, "invalid JSON array"))
			return
		}

		results := make([]json.RawMessage, 0, len(rawMessages))
		for _, msg := range rawMessages {
			results = append(results, h.server.Handle(msg, perms))
		}

		resp, _ := json.Marshal(results)
		c.Data(http.StatusOK, "application/json", resp)
	} else {
		result := h.server.Handle(body, perms)
		c.Data(http.StatusOK, "application/json", result)
	}
}
