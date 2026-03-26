// Package api contains definitions for LarkSuite API
package api

import (
	"strconv"
	"time"
)

// UnixTime is a custom type to handle Unix timestamp from JSON
type UnixTime time.Time

// UnmarshalJSON implements json.Unmarshaler
func (t *UnixTime) UnmarshalJSON(data []byte) error {
	// Remove quotes if present
	str := string(data)
	if len(str) >= 2 && str[0] == '"' && str[len(str)-1] == '"' {
		str = str[1 : len(str)-1]
	}

	// Parse as int64 (Unix timestamp)
	ts, err := strconv.ParseInt(str, 10, 64)
	if err != nil {
		// Try parsing as RFC3339 string as fallback
		var tt time.Time
		tt, err = time.Parse(time.RFC3339, str)
		if err != nil {
			return err
		}
		*t = UnixTime(tt)
		return nil
	}

	*t = UnixTime(time.Unix(ts, 0))
	return nil
}

// Time returns the time.Time value
func (t UnixTime) Time() time.Time {
	return time.Time(t)
}

// File represents a file or folder in LarkSuite Drive
type File struct {
	Token        string   `json:"token"`
	Name         string   `json:"name"`
	Type         string   `json:"type"` // "file" or "folder"
	ParentToken  string   `json:"parent_token"`
	Size         int64    `json:"size"`
	MimeType     string   `json:"mime_type"`
	ModifiedTime UnixTime `json:"modified_time"`
	CreatedTime  UnixTime `json:"created_time"`
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
		Files         []File `json:"files"`
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

// RootFolderResponse is the response for getting root folder meta
type RootFolderResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Token string `json:"token"`
		Name  string `json:"name"`
	} `json:"data"`
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

// AppAccessTokenResponse is the response for getting app access token
type AppAccessTokenResponse struct {
	Code           int    `json:"code"`
	Msg            string `json:"msg"`
	AppAccessToken string `json:"app_access_token"`
	Expire         int    `json:"expire"`
}

// LoginTokenResponse is the response for getting login token (temporary auth code)
type LoginTokenResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
	Data struct {
		Code string `json:"code"`
	} `json:"data"`
}

// UserAccessTokenResponse is the response for getting user access token
type UserAccessTokenResponse struct {
	Code            int    `json:"code"`
	Msg             string `json:"msg"`
	AccessToken     string `json:"access_token"`
	RefreshToken    string `json:"refresh_token"`
	TokenType       string `json:"token_type"`
	ExpiresIn       int    `json:"expires_in"`
	UserAccessToken string `json:"user_access_token"`
}

// CreateFolderRequest is the request to create a folder
type CreateFolderRequest struct {
	Name        string `json:"name"`
	FolderToken string `json:"folder_token,omitempty"`
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

// CollaboratorRequest is the request to add a collaborator
type CollaboratorRequest struct {
	Members []Member `json:"members"`
}

// Member represents a collaborator member
type Member struct {
	MemberID   string `json:"member_id"`
	MemberType string `json:"member_type"` // "email", "openid", "openchat", "opendepartmentid", "userid"
	Perm       string `json:"perm"`        // "view", "edit", "full_access"
}

// CollaboratorResponse is the response for adding a collaborator
type CollaboratorResponse struct {
	Code int    `json:"code"`
	Msg  string `json:"msg"`
}
