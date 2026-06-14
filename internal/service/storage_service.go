package service

import "mime/multipart"

// FileUploader 定义了文件上传的行为
// 以后无论是本地存储还是云存储，都只要实现这个接口即可
type FileUploader interface {
	// 接收一个文件，返回最终的访问 URL 和 错误信息
	SaveFile(file *multipart.FileHeader) (url string, err error)
}