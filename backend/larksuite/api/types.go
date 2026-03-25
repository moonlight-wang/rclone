// Package api contains definitions for LarkSuite API
package api

import (
	"time"
)

// File represents a file or folder in LarkSuite Drive
type File struct {
	Token       string    `json:"token"`
	Name        string    `json:"name"`
	Type        string    `json:"type"` // "file" or "folder"
	ParentToken string    `json:"parent_token"`
	Size        int64     `json:"size"`
	MimeType    string    `json:"mime_type"`
	ModifiedTime time.Time `json:"modified_time"`
	CreatedTime  time.Time `json:"created_time"`
}

// IsDir returns true if the file is a directory
func (f *File) IsDir() bool {
	return f.Type == "folder"
}

// FileListResponse is the response for listing files
type FileListResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Files      []File `json:"files"`
		NextPageToken string `json:"next_page_token"`
	} `json:"data"`
}

// FileResponse is the response for a single file operation
type FileResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data File   `json:"data"`
}

// UploadInfo contains information about an upload
type UploadInfo struct {
	Token    string `json:"token"`
	Name     string `json:"name"`
	Size     int64  `json:"size"`
	MimeType string `json:"mime_type"`
}

// ErrorResponse represents an error from the API
type ErrorResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}

// Error implements the error interface
func (e *ErrorResponse) Error() string {
	return e.Msg
}

// TenantAccessTokenResponse is the response for getting tenant access token
type TenantAccessTokenResponse struct {
	Code              int    `json:"code"`
	Msg               string `json:"msg"`
	TenantAccessToken string `json:"tenant_access_token"`
	Expire            int    `json:"expire"`
}

// UserAccessTokenResponse is the response for getting user access token
type UserAccessTokenResponse struct {
	Code            int    `json:"code"`
	Msg             string `json:"msg"`
	UserAccessToken string `json:"user_access_token"`
	Expire          int    `json:"expire"`
}

// CreateFolderRequest is the request to create a folder
type CreateFolderRequest struct {
	Name        string `json:"name"`
	ParentToken string `json:"parent_token,omitempty"`
}

// CreateFolderResponse is the response for creating a folder
type CreateFolderResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	} `json:"data"`
}

// UploadFileResponse is the response for uploading a file
type UploadFileResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Token    string `json:"token"`
		Name     string `json:"name"`
		Size     int64  `json:"size"`
		MimeType string `json:"mime_type"`
	} `json:"data"`
}

// PrepareUploadResponse is the response for preparing a file upload
type PrepareUploadResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		UploadToken string `json:"upload_token"`
		BlockSize   int64  `json:"block_size"`
		BlockNum    int    `json:"block_num"`
	} `json:"data"`
}
