package infrastructure

import (
	"fmt"
	"io"
	"mime/multipart"
	"os"
	"path/filepath"
	"time"
)

// LocalStorage 本地存储的结构体
type LocalStorage struct {
	UploadDir string // 例如 "./uploads"
}

func NewLocalStorage(dir string) *LocalStorage {
	// 确保上传目录存在
	os.MkdirAll(dir, os.ModePerm)
	return &LocalStorage{UploadDir: dir}
}

// SaveFile 实现了 service.FileUploader 接口
func (s *LocalStorage) SaveFile(file *multipart.FileHeader) (string, error) {
	// 1. 打开上传的文件
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	// 2. 生成唯一文件名 (时间戳 + 原始扩展名)
	ext := filepath.Ext(file.Filename)
	newFileName := fmt.Sprintf("%d%s", time.Now().UnixNano(), ext)
	
	// 3. 拼接保存路径
	dstPath := filepath.Join(s.UploadDir, newFileName)
	dst, err := os.Create(dstPath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	// 4. 拷贝文件内容
	if _, err = io.Copy(dst, src); err != nil {
		return "", err
	}

	// 5. 返回图片的相对访问路径 (供给前端使用)
	return "/uploads/" + newFileName, nil
}