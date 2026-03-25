// Package larksuite provides an interface to the LarkSuite Drive storage system.
package larksuite

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"

	"github.com/rclone/rclone/backend/larksuite/api"
	"github.com/rclone/rclone/fs"
	"github.com/rclone/rclone/fs/config"
	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fs/config/configstruct"
	"github.com/rclone/rclone/fs/fshttp"
	"github.com/rclone/rclone/fs/hash"
	"github.com/rclone/rclone/lib/dircache"
	"github.com/rclone/rclone/lib/encoder"
	"github.com/rclone/rclone/lib/pacer"
)

const (
	apiBaseURL          = "https://open.larksuite.com/open-apis"
	minSleep            = 100 * time.Millisecond
	maxSleep            = 2 * time.Second
	decayConstant       = 2
	defaultChunkSize    = 4 * fs.Mebi
	maxUploadSizeSimple = 20 * fs.Mebi // Max size for simple upload
)

// Register with Fs
func init() {
	fs.Register(&fs.RegInfo{
		Name:        "larksuite",
		Description: "LarkSuite Drive",
		NewFs:       NewFs,
		Config: func(ctx context.Context, name string, m configmap.Mapper, config fs.ConfigIn) (*fs.ConfigOut, error) {
			opt := new(Options)
			err := configstruct.Set(m, opt)
			if err != nil {
				return nil, fmt.Errorf("couldn't parse config into struct: %w", err)
			}

			if opt.AppID == "" || opt.AppSecret == "" {
				return fs.ConfigError("", "App ID and App Secret are required")
			}

			// Test the credentials by getting a tenant access token
			tempFs := &Fs{
				opt: *opt,
			}
			tempFs.httpClient = fshttp.NewClient(ctx)
			_, err = tempFs.getTenantAccessToken(ctx)
			if err != nil {
				return fs.ConfigError("", fmt.Sprintf("Failed to authenticate: %v", err))
			}

			return nil, nil
		},
		Options: []fs.Option{{
			Name:     "app_id",
			Help:     "LarkSuite App ID\nGet from https://open.larksuite.com/app",
			Required: true,
		}, {
			Name:       "app_secret",
			Help:       "LarkSuite App Secret",
			Required:   true,
			IsPassword: true,
		}, {
			Name:     "root_folder_id",
			Help:     "Root folder token (leave empty for root)",
			Advanced: true,
			Default:  "",
		}, {
			Name:     config.ConfigEncoding,
			Help:     config.ConfigEncodingHelp,
			Advanced: true,
			Default: (encoder.Display |
				encoder.EncodeBackSlash |
				encoder.EncodeInvalidUtf8),
		}},
	})
}

// Options defines the configuration for this backend
type Options struct {
	AppID        string               `config:"app_id"`
	AppSecret    string               `config:"app_secret"`
	RootFolderID string               `config:"root_folder_id"`
	Enc          encoder.MultiEncoder `config:"encoding"`
}

// Fs represents a remote LarkSuite Drive server
type Fs struct {
	name        string             // name of this remote
	root        string             // the path we are working on
	features    *fs.Features       // optional features
	opt         Options            // options for this Fs
	httpClient  *http.Client       // HTTP client
	pacer       *fs.Pacer          // To pace the API calls
	dirCache    *dircache.DirCache // Map of directory path to directory id
	token       string             // tenant access token
	tokenExpiry time.Time          // token expiry time
}

// Object describes a LarkSuite Drive object
type Object struct {
	fs       *Fs    // what this object is part of
	remote   string // The remote path
	token    string // File token
	size     int64  // size of the object
	modTime  time.Time
	mimeType string
}

// ------------------------------------------------------------
// Fs Interface Implementation
// ------------------------------------------------------------

// Name of the remote (as passed into NewFs)
func (f *Fs) Name() string {
	return f.name
}

// Root of the remote (as passed into NewFs)
func (f *Fs) Root() string {
	return f.root
}

// String converts this Fs to a string
func (f *Fs) String() string {
	return fmt.Sprintf("LarkSuite Drive root '%s'", f.root)
}

// Features returns the optional features of this Fs
func (f *Fs) Features() *fs.Features {
	return f.features
}

// Precision return the precision of this Fs
func (f *Fs) Precision() time.Duration {
	return time.Second
}

// Hashes returns the supported hash sets.
func (f *Fs) Hashes() hash.Set {
	return hash.Set(hash.None)
}

// parsePath parses a larksuite 'url'
func parsePath(root string) string {
	root = strings.Trim(root, "/")
	return root
}

// NewFs constructs an Fs from the path, container:path
func NewFs(ctx context.Context, name, root string, m configmap.Mapper) (fs.Fs, error) {
	opt := new(Options)
	err := configstruct.Set(m, opt)
	if err != nil {
		return nil, err
	}

	root = parsePath(root)

	f := &Fs{
		name:       name,
		root:       root,
		opt:        *opt,
		httpClient: fshttp.NewClient(ctx),
		pacer:      fs.NewPacer(ctx, pacer.NewDefault(pacer.MinSleep(minSleep), pacer.MaxSleep(maxSleep), pacer.DecayConstant(decayConstant))),
	}

	f.features = (&fs.Features{
		CaseInsensitive:         true,
		CanHaveEmptyDirectories: true,
		ReadMimeType:            true,
	}).Fill(ctx, f)

	// Set up dir cache
	rootID := opt.RootFolderID
	if rootID == "" {
		rootID = "0" // Use "0" as special root identifier
	}
	f.dirCache = dircache.New(root, rootID, f)

	// Find the current root
	err = f.dirCache.FindRoot(ctx, false)
	if err != nil {
		// Assume it is a file
		newRoot, remote := dircache.SplitPath(root)
		tempF := *f
		tempF.dirCache = dircache.New(newRoot, rootID, &tempF)
		tempF.root = newRoot
		// Make new Fs which is the parent
		err = tempF.dirCache.FindRoot(ctx, false)
		if err != nil {
			// No root so return old f
			return f, nil
		}
		_, err := tempF.NewObject(ctx, remote)
		if err != nil {
			// unable to list folder so return old f
			return f, nil
		}
		f.dirCache = tempF.dirCache
		f.root = tempF.root
		return f, fs.ErrorIsFile
	}

	return f, nil
}

// getTenantAccessToken gets a tenant access token for app authentication
func (f *Fs) getTenantAccessToken(ctx context.Context) (string, error) {
	// Check if we have a valid cached token
	if f.token != "" && time.Now().Before(f.tokenExpiry) {
		return f.token, nil
	}

	url := apiBaseURL + "/auth/v3/tenant_access_token/internal"
	payload := fmt.Sprintf(`{"app_id":"%s","app_secret":"%s"}`, f.opt.AppID, f.opt.AppSecret)

	req, err := http.NewRequestWithContext(ctx, "POST", url, strings.NewReader(payload))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")

	var resp api.TenantAccessTokenResponse
	err = f.pacer.Call(func() (bool, error) {
		res, err := f.httpClient.Do(req)
		if err != nil {
			return true, err
		}
		defer res.Body.Close()

		if res.StatusCode >= 500 {
			return true, fmt.Errorf("server error: %d", res.StatusCode)
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			return false, err
		}

		// Parse response
		if err := json.Unmarshal(body, &resp); err != nil {
			return false, err
		}

		if resp.Code != 0 {
			return false, fmt.Errorf("API error: %s", resp.Msg)
		}

		return false, nil
	})

	if err != nil {
		return "", err
	}

	f.token = resp.TenantAccessToken
	f.tokenExpiry = time.Now().Add(time.Duration(resp.Expire-60) * time.Second) // Refresh 1 minute before expiry
	return f.token, nil
}

// callAPI makes an authenticated API call
func (f *Fs) callAPI(ctx context.Context, method, url string, body io.Reader, result interface{}) error {
	token, err := f.getTenantAccessToken(ctx)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	return f.pacer.Call(func() (bool, error) {
		res, err := f.httpClient.Do(req)
		if err != nil {
			return true, err
		}
		defer res.Body.Close()

		if res.StatusCode >= 500 {
			return true, fmt.Errorf("server error: %d", res.StatusCode)
		}

		respBody, err := io.ReadAll(res.Body)
		if err != nil {
			return false, err
		}

		if res.StatusCode != http.StatusOK && res.StatusCode != http.StatusCreated {
			return false, fmt.Errorf("API error: %s", string(respBody))
		}

		if result != nil && len(respBody) > 0 {
			if err := json.Unmarshal(respBody, result); err != nil {
				return false, err
			}
		}

		return false, nil
	})
}

// FindLeaf finds a directory of name leaf in the folder with ID pathID
func (f *Fs) FindLeaf(ctx context.Context, pathID, leaf string) (string, bool, error) {
	if pathID == "0" && leaf == "" {
		return pathID, true, nil
	}

	files, err := f.listFiles(ctx, pathID)
	if err != nil {
		return "", false, err
	}

	for _, file := range files {
		if f.opt.Enc.ToStandardName(file.Name) == leaf {
			if file.IsDir() {
				return file.Token, true, nil
			}
			return "", false, fs.ErrorIsFile
		}
	}

	return "", false, nil
}

// CreateDir makes a directory with pathID as parent and name leaf
func (f *Fs) CreateDir(ctx context.Context, pathID, leaf string) (string, error) {
	url := apiBaseURL + "/drive/v1/files/create_folder"

	reqBody := api.CreateFolderRequest{
		Name:        f.opt.Enc.FromStandardName(leaf),
		ParentToken: pathID,
	}

	if pathID == "0" {
		reqBody.ParentToken = ""
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	var resp api.CreateFolderResponse
	err = f.callAPI(ctx, "POST", url, bytes.NewReader(jsonBody), &resp)
	if err != nil {
		return "", err
	}

	if resp.Code != 0 {
		return "", fmt.Errorf("failed to create folder: %s", resp.Msg)
	}

	return resp.Data.Token, nil
}

// listFiles lists files in a folder
func (f *Fs) listFiles(ctx context.Context, folderToken string) ([]api.File, error) {
	var allFiles []api.File
	pageToken := ""

	for {
		apiURL := apiBaseURL + "/drive/v1/files?folder_token=" + url.QueryEscape(folderToken)
		if pageToken != "" {
			apiURL += "&page_token=" + url.QueryEscape(pageToken)
		}

		var resp api.FileListResponse
		err := f.callAPI(ctx, "GET", apiURL, nil, &resp)
		if err != nil {
			return nil, err
		}

		if resp.Code != 0 {
			return nil, fmt.Errorf("failed to list files: %s", resp.Msg)
		}

		allFiles = append(allFiles, resp.Data.Files...)

		if resp.Data.NextPageToken == "" {
			break
		}
		pageToken = resp.Data.NextPageToken
	}

	return allFiles, nil
}

// List the objects and directories in dir into entries
func (f *Fs) List(ctx context.Context, dir string) (entries fs.DirEntries, err error) {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return nil, err
	}

	files, err := f.listFiles(ctx, directoryID)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		remote := path.Join(dir, f.opt.Enc.ToStandardName(file.Name))
		if file.IsDir() {
			f.dirCache.Put(remote, file.Token)
			d := fs.NewDir(remote, file.ModifiedTime)
			entries = append(entries, d)
		} else {
			o, err := f.newObjectWithInfo(ctx, remote, &file)
			if err != nil {
				return nil, err
			}
			entries = append(entries, o)
		}
	}

	return entries, nil
}

// NewObject finds the Object at remote
func (f *Fs) NewObject(ctx context.Context, remote string) (fs.Object, error) {
	return f.readMetaDataForPath(ctx, remote)
}

// readMetaDataForPath reads the metadata for a path
func (f *Fs) readMetaDataForPath(ctx context.Context, remote string) (*Object, error) {
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, false)
	if err != nil {
		if err == fs.ErrorDirNotFound {
			return nil, fs.ErrorObjectNotFound
		}
		return nil, err
	}

	files, err := f.listFiles(ctx, directoryID)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if f.opt.Enc.ToStandardName(file.Name) == leaf {
			if file.IsDir() {
				return nil, fs.ErrorIsDir
			}
			return f.newObjectWithInfo(ctx, remote, &file)
		}
	}

	return nil, fs.ErrorObjectNotFound
}

// newObjectWithInfo creates a new Object with the given info
func (f *Fs) newObjectWithInfo(ctx context.Context, remote string, info *api.File) (*Object, error) {
	o := &Object{
		fs:       f,
		remote:   remote,
		token:    info.Token,
		size:     info.Size,
		modTime:  info.ModifiedTime,
		mimeType: info.MimeType,
	}
	return o, nil
}

// Put uploads a file
func (f *Fs) Put(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	existingObj, err := f.NewObject(ctx, src.Remote())
	switch err {
	case nil:
		return existingObj, existingObj.Update(ctx, in, src, options...)
	case fs.ErrorObjectNotFound:
		return f.putUnchecked(ctx, in, src, src.Remote(), options...)
	default:
		return nil, err
	}
}

// PutUnchecked uploads the object without checking if it exists
func (f *Fs) PutUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) (fs.Object, error) {
	remote := src.Remote()
	return f.putUnchecked(ctx, in, src, remote, options...)
}

// putUnchecked uploads the object without checking if it exists
func (f *Fs) putUnchecked(ctx context.Context, in io.Reader, src fs.ObjectInfo, remote string, options ...fs.OpenOption) (*Object, error) {
	size := src.Size()

	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	// Use simple upload for small files, resumable for large files
	if size <= int64(maxUploadSizeSimple) {
		return f.uploadSimple(ctx, in, leaf, directoryID, size, src.ModTime(ctx))
	}
	return f.uploadResumable(ctx, in, leaf, directoryID, size, src.ModTime(ctx))
}

// uploadSimple uploads a small file using simple upload
func (f *Fs) uploadSimple(ctx context.Context, in io.Reader, name, parentID string, size int64, modTime time.Time) (*Object, error) {
	// Implementation for simple upload using LarkSuite's upload API
	url := apiBaseURL + "/drive/v1/files/upload_all"

	token, err := f.getTenantAccessToken(ctx)
	if err != nil {
		return nil, err
	}

	// Read file content
	content, err := io.ReadAll(in)
	if err != nil {
		return nil, err
	}

	// Build multipart form data
	var b bytes.Buffer
	writer := multipart.NewWriter(&b)

	// Add file
	part, err := writer.CreateFormFile("file", f.opt.Enc.FromStandardName(name))
	if err != nil {
		return nil, err
	}
	_, err = part.Write(content)
	if err != nil {
		return nil, err
	}

	// Add parent token
	err = writer.WriteField("parent_token", parentID)
	if err != nil {
		return nil, err
	}

	err = writer.Close()
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, &b)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	var resp api.UploadFileResponse
	err = f.pacer.Call(func() (bool, error) {
		res, err := f.httpClient.Do(req)
		if err != nil {
			return true, err
		}
		defer res.Body.Close()

		if res.StatusCode >= 500 {
			return true, fmt.Errorf("server error: %d", res.StatusCode)
		}

		body, err := io.ReadAll(res.Body)
		if err != nil {
			return false, err
		}

		if err := json.Unmarshal(body, &resp); err != nil {
			return false, err
		}

		if resp.Code != 0 {
			return false, fmt.Errorf("upload failed: %s", resp.Msg)
		}

		return false, nil
	})

	if err != nil {
		return nil, err
	}

	// Create object from response
	file := &api.File{
		Token:        resp.Data.Token,
		Name:         resp.Data.Name,
		Type:         "file",
		ParentToken:  parentID,
		Size:         resp.Data.Size,
		MimeType:     resp.Data.MimeType,
		ModifiedTime: modTime,
	}

	// Build the remote path from the current root and file name
	remote := path.Join(f.root, name)

	return f.newObjectWithInfo(ctx, remote, file)
}

// uploadResumable uploads a large file using resumable upload
func (f *Fs) uploadResumable(ctx context.Context, in io.Reader, name, parentID string, size int64, modTime time.Time) (*Object, error) {
	// Implementation for resumable upload
	// This is a placeholder - actual implementation would use LarkSuite's resumable upload API
	return nil, fmt.Errorf("resumable upload not yet implemented")
}

// Mkdir creates the directory if it doesn't exist
func (f *Fs) Mkdir(ctx context.Context, dir string) error {
	_, err := f.dirCache.FindDir(ctx, dir, true)
	return err
}

// Rmdir deletes an empty directory
func (f *Fs) Rmdir(ctx context.Context, dir string) error {
	directoryID, err := f.dirCache.FindDir(ctx, dir, false)
	if err != nil {
		return err
	}

	// Check if directory is empty
	files, err := f.listFiles(ctx, directoryID)
	if err != nil {
		return err
	}

	if len(files) > 0 {
		return fmt.Errorf("directory not empty")
	}

	// Delete the directory
	url := apiBaseURL + "/drive/v1/files/" + directoryID
	err = f.callAPI(ctx, "DELETE", url, nil, nil)
	if err != nil {
		return err
	}

	f.dirCache.FlushDir(dir)
	return nil
}

// DirCacheFlush resets the directory cache - used in testing
func (f *Fs) DirCacheFlush() {
	f.dirCache.ResetRoot()
}

// Move src to this remote using server-side move operations
func (f *Fs) Move(ctx context.Context, src fs.Object, remote string) (fs.Object, error) {
	srcObj, ok := src.(*Object)
	if !ok {
		fs.Debugf(src, "Can't move - not same remote type")
		return nil, fs.ErrorCantMove
	}

	// Find the destination directory
	leaf, directoryID, err := f.dirCache.FindPath(ctx, remote, true)
	if err != nil {
		return nil, err
	}

	// Move the file using API
	url := apiBaseURL + "/drive/v1/files/" + srcObj.token + "/move"
	payload := fmt.Sprintf(`{"target_folder_token":"%s","name":"%s"}`, directoryID, f.opt.Enc.FromStandardName(leaf))

	err = f.callAPI(ctx, "POST", url, strings.NewReader(payload), nil)
	if err != nil {
		return nil, err
	}

	return f.NewObject(ctx, remote)
}

// DirMove moves src, srcRemote to this remote using server-side move operations
func (f *Fs) DirMove(ctx context.Context, src fs.Fs, srcRemote, dstRemote string) error {
	srcFs, ok := src.(*Fs)
	if !ok {
		fs.Debugf(srcFs, "Can't move directory - not same remote type")
		return fs.ErrorCantDirMove
	}

	srcID, err := srcFs.dirCache.FindDir(ctx, srcRemote, false)
	if err != nil {
		return err
	}

	dstParentID, err := f.dirCache.FindDir(ctx, path.Dir(dstRemote), true)
	if err != nil {
		return err
	}

	leaf := path.Base(dstRemote)

	// Move the folder using API
	url := apiBaseURL + "/drive/v1/files/" + srcID + "/move"
	payload := fmt.Sprintf(`{"target_folder_token":"%s","name":"%s"}`, dstParentID, f.opt.Enc.FromStandardName(leaf))

	return f.callAPI(ctx, "POST", url, strings.NewReader(payload), nil)
}

// ------------------------------------------------------------
// Object Interface Implementation
// ------------------------------------------------------------

// Fs returns the parent Fs
func (o *Object) Fs() fs.Info {
	return o.fs
}

// Remote returns the remote path
func (o *Object) Remote() string {
	return o.remote
}

// String returns a string representation
func (o *Object) String() string {
	if o == nil {
		return "<nil>"
	}
	return o.remote
}

// Hash returns the hash of the object
func (o *Object) Hash(ctx context.Context, t hash.Type) (string, error) {
	return "", hash.ErrUnsupported
}

// Size returns the size of the object
func (o *Object) Size() int64 {
	return o.size
}

// ModTime returns the modification time
func (o *Object) ModTime(ctx context.Context) time.Time {
	return o.modTime
}

// SetModTime sets the modification time
func (o *Object) SetModTime(ctx context.Context, t time.Time) error {
	return fs.ErrorNotImplemented
}

// Storable returns whether this object is storable
func (o *Object) Storable() bool {
	return true
}

// MimeType returns the mime type
func (o *Object) MimeType(ctx context.Context) string {
	return o.mimeType
}

// Open opens the file for read
func (o *Object) Open(ctx context.Context, options ...fs.OpenOption) (io.ReadCloser, error) {
	var offset, limit int64 = 0, -1
	for _, option := range options {
		switch x := option.(type) {
		case *fs.SeekOption:
			offset = x.Offset
		case *fs.RangeOption:
			offset, limit = x.Decode(o.size)
		default:
			if option.Mandatory() {
				fs.Logf(o, "Unsupported mandatory option: %v", option)
			}
		}
	}

	// Get download URL
	url := apiBaseURL + "/drive/v1/files/" + o.token + "/download"

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	token, err := o.fs.getTenantAccessToken(ctx)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	if offset > 0 || limit >= 0 {
		rangeHeader := fmt.Sprintf("bytes=%d-", offset)
		if limit >= 0 {
			rangeHeader += fmt.Sprintf("%d", offset+limit-1)
		}
		req.Header.Set("Range", rangeHeader)
	}

	resp, err := o.fs.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		resp.Body.Close()
		return nil, fmt.Errorf("download failed: %d", resp.StatusCode)
	}

	return resp.Body, nil
}

// Update updates the object with the contents of the reader
func (o *Object) Update(ctx context.Context, in io.Reader, src fs.ObjectInfo, options ...fs.OpenOption) error {
	// Delete the old file and upload a new one
	err := o.Remove(ctx)
	if err != nil {
		return err
	}

	newObj, err := o.fs.putUnchecked(ctx, in, src, o.remote, options...)
	if err != nil {
		return err
	}

	*o = *newObj
	return nil
}

// Remove deletes the object
func (o *Object) Remove(ctx context.Context) error {
	url := apiBaseURL + "/drive/v1/files/" + o.token
	return o.fs.callAPI(ctx, "DELETE", url, nil, nil)
}

// Check the interfaces are satisfied
var (
	_ fs.Fs              = (*Fs)(nil)
	_ fs.DirCacheFlusher = (*Fs)(nil)
	_ dircache.DirCacher = (*Fs)(nil)
	_ fs.PutUncheckeder  = (*Fs)(nil)
	_ fs.Mover           = (*Fs)(nil)
	_ fs.DirMover        = (*Fs)(nil)
	_ fs.Object          = (*Object)(nil)
	_ fs.MimeTyper       = (*Object)(nil)
)
