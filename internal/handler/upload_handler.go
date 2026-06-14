package handler

import (
	"VMQ-api-go/internal/service"
	"net/http"

	"github.com/gin-gonic/gin"
)

// UploadHandler 上传控制器
type UploadHandler struct {
	// 依赖注入：这里放的是接口，而不是具体的 LocalStorage
	uploader service.FileUploader
}

func NewUploadHandler(u service.FileUploader) *UploadHandler {
	return &UploadHandler{uploader: u}
}

// HandleImageUpload 处理前端 Ant Design Pro 发来的请求
func (h *UploadHandler) HandleImageUpload(c *gin.Context) {
	// 1. 解析前端传来的文件（"file" 必须和前端 customRequest 中 append 的字段名一致）
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "获取上传文件失败"})
		return
	}

	// 2. 调用接口保存文件
	url, err := h.uploader.SaveFile(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "文件保存失败"})
		return
	}

	// 3. 返回前端需要的 JSON 格式
	c.JSON(http.StatusOK, gin.H{
		"status": "done",
		"url":    url,
	})
}
