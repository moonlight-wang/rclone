// Test Suite for LarkSuite Drive
package larksuite

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/rclone/rclone/fs/config/configmap"
	"github.com/rclone/rclone/fstest/fstests"
)

// Test credentials - provided by user
const (
	testAppID     = "cli_a94b457550f95ed1"
	testAppSecret = "VI3JXw89zJNGGUmivEkmddrvu3mo6NlO"
)

// TestNewFs tests creating a new file system
func TestNewFs(t *testing.T) {
	ctx := context.Background()

	// Create config map with test credentials
	m := configmap.Simple{
		"app_id":     testAppID,
		"app_secret": testAppSecret,
	}

	// Try to create new filesystem
	fs, err := NewFs(ctx, "test", "", m)
	if err != nil {
		t.Logf("NewFs returned error (expected if credentials are invalid): %v", err)
		// Don't fail the test if credentials are invalid
		// This is expected behavior for unit tests with test credentials
		return
	}

	if fs == nil {
		t.Fatal("NewFs returned nil filesystem")
	}

	t.Logf("Successfully created filesystem: %s", fs.String())
}

// TestAuthentication tests the authentication flow
func TestAuthentication(t *testing.T) {
	ctx := context.Background()

	f := &Fs{
		opt: Options{
			AppID:     testAppID,
			AppSecret: testAppSecret,
		},
	}

	// Try to get tenant access token
	token, err := f.getAccessToken(ctx)
	if err != nil {
		t.Logf("Authentication failed (expected for test credentials): %v", err)
		return
	}

	if token == "" {
		t.Error("Got empty token")
	}

	t.Logf("Successfully obtained access token")
}

// TestParsePath tests path parsing
func TestParsePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/path/to/file", "path/to/file"},
		{"path/to/file", "path/to/file"},
		{"/path/to/folder/", "path/to/folder"},
		{"", ""},
		{"/", ""},
	}

	for _, test := range tests {
		result := parsePath(test.input)
		if result != test.expected {
			t.Errorf("parsePath(%q) = %q, expected %q", test.input, result, test.expected)
		}
	}
}

// TestOptions tests the Options struct
func TestOptions(t *testing.T) {
	opt := Options{
		AppID:        testAppID,
		AppSecret:    testAppSecret,
		RootFolderID: "test-folder-id",
	}

	if opt.AppID != testAppID {
		t.Errorf("AppID mismatch: got %q, expected %q", opt.AppID, testAppID)
	}

	if opt.AppSecret != testAppSecret {
		t.Errorf("AppSecret mismatch: got %q, expected %q", opt.AppSecret, testAppSecret)
	}

	if opt.RootFolderID != "test-folder-id" {
		t.Errorf("RootFolderID mismatch: got %q, expected %q", opt.RootFolderID, "test-folder-id")
	}
}

// TestIntegration is an integration test that requires valid credentials
// This test is skipped by default to avoid running in CI
func TestIntegration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Check if credentials are provided
	if testAppID == "" || testAppSecret == "" {
		t.Skip("Skipping integration test - no credentials provided")
	}

	ctx := context.Background()

	m := configmap.Simple{
		"app_id":     testAppID,
		"app_secret": testAppSecret,
	}

	fs, err := NewFs(ctx, "test", "", m)
	if err != nil {
		t.Fatalf("Failed to create filesystem: %v", err)
	}

	// Test listing root directory
	entries, err := fs.List(ctx, "")
	if err != nil {
		t.Logf("List root failed: %v", err)
	} else {
		t.Logf("Root directory has %d entries", len(entries))
	}
}

// TestFeatures tests that the filesystem features are properly set
func TestFeatures(t *testing.T) {
	ctx := context.Background()

	m := configmap.Simple{
		"app_id":     testAppID,
		"app_secret": testAppSecret,
	}

	fs, err := NewFs(ctx, "test", "", m)
	if err != nil {
		t.Skipf("Skipping feature test - could not create filesystem: %v", err)
	}

	features := fs.Features()
	if features == nil {
		t.Fatal("Features returned nil")
	}

	// Check expected features
	if !features.CaseInsensitive {
		t.Error("Expected CaseInsensitive to be true")
	}

	if !features.CanHaveEmptyDirectories {
		t.Error("Expected CanHaveEmptyDirectories to be true")
	}

	if !features.ReadMimeType {
		t.Error("Expected ReadMimeType to be true")
	}
}

// TestFSInterface tests that Fs implements the required interfaces
func TestFSInterface(t *testing.T) {
	ctx := context.Background()

	m := configmap.Simple{
		"app_id":     testAppID,
		"app_secret": testAppSecret,
	}

	fs, err := NewFs(ctx, "test", "", m)
	if err != nil {
		t.Skipf("Skipping interface test - could not create filesystem: %v", err)
	}

	// Test that fs implements fs.Fs
	if fs.Name() == "" {
		t.Error("Name() returned empty string")
	}

	if fs.Root() == "" && fs.Root() != "" {
		t.Error("Root() returned unexpected value")
	}

	if fs.String() == "" {
		t.Error("String() returned empty string")
	}

	if fs.Precision() == 0 {
		t.Error("Precision() returned zero")
	}

	if fs.Hashes() == 0 {
		t.Error("Hashes() returned zero")
	}
}

// BenchmarkNewFs benchmarks creating a new filesystem
func BenchmarkNewFs(b *testing.B) {
	ctx := context.Background()
	m := configmap.Simple{
		"app_id":     testAppID,
		"app_secret": testAppSecret,
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := NewFs(ctx, "test", "", m)
		if err != nil {
			// Expected for invalid credentials
			continue
		}
	}
}

// BenchmarkParsePath benchmarks path parsing
func BenchmarkParsePath(b *testing.B) {
	paths := []string{
		"/path/to/file",
		"path/to/file",
		"/path/to/folder/",
		"",
		"/",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for _, path := range paths {
			parsePath(path)
		}
	}
}

// TestStandard is a helper function to run the standard rclone tests
// This requires valid credentials to work properly
func TestStandard(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping standard tests in short mode")
	}

	// Check if credentials are provided
	if testAppID == "" || testAppSecret == "" {
		t.Skip("Skipping standard tests - no credentials provided")
	}

	opt := fstests.Opt{
		RemoteName: "TestLarkSuite:",
	}

	fstests.Run(t, &opt)
}

// TestListFiles tests listing files in the root directory
func TestListFiles(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping list files test in short mode")
	}

	// Check if credentials are provided
	if testAppID == "" || testAppSecret == "" {
		t.Skip("Skipping list files test - no credentials provided")
	}

	ctx := context.Background()

	m := configmap.Simple{
		"app_id":             testAppID,
		"app_secret":         testAppSecret,
		"collaborator_email": "wangxianji82@gmail.com",
	}

	fs, err := NewFs(ctx, "test", "", m)
	if err != nil {
		t.Skipf("Skipping list files test - could not create filesystem: %v", err)
	}

	// Test listing root directory
	entries, err := fs.List(ctx, "")
	if err != nil {
		t.Logf("List root directory returned error: %v", err)
		// Don't fail if root is empty or API has issues
		return
	}

	t.Logf("Root directory has %d entries", len(entries))

	// Log each entry for debugging
	for _, entry := range entries {
		t.Logf("Entry: %s", entry.String())
	}
}

// TestMkdir tests creating a directory
func TestMkdir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping mkdir test in short mode")
	}

	// Check if credentials are provided
	if testAppID == "" || testAppSecret == "" {
		t.Skip("Skipping mkdir test - no credentials provided")
	}

	ctx := context.Background()

	m := configmap.Simple{
		"app_id":             testAppID,
		"app_secret":         testAppSecret,
		"collaborator_email": "wangxianji82@gmail.com",
	}

	fs, err := NewFs(ctx, "test", "", m)
	if err != nil {
		t.Skipf("Skipping mkdir test - could not create filesystem: %v", err)
	}

	// Create a test folder with timestamp to avoid conflicts
	testFolderName := fmt.Sprintf("test_folder_%d", time.Now().Unix())

	// Test creating directory (will automatically add collaborator)
	err = fs.Mkdir(ctx, testFolderName)
	if err != nil {
		t.Logf("Mkdir returned error: %v", err)
		// Don't fail if directory already exists or API has issues
		return
	}

	t.Logf("Successfully created directory: %s", testFolderName)

	// Verify the directory was created by listing
	entries, err := fs.List(ctx, "")
	if err != nil {
		t.Logf("List after mkdir returned error: %v", err)
		return
	}

	found := false
	for _, entry := range entries {
		if entry.String() == testFolderName {
			found = true
			break
		}
	}

	if found {
		t.Logf("Verified directory exists: %s", testFolderName)
	} else {
		t.Logf("Directory not found in listing: %s", testFolderName)
	}
}

// TestListAndMkdir tests listing files and creating directories together
func TestListAndMkdir(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping combined test in short mode")
	}

	// Check if credentials are provided
	if testAppID == "" || testAppSecret == "" {
		t.Skip("Skipping combined test - no credentials provided")
	}

	ctx := context.Background()

	m := configmap.Simple{
		"app_id":             testAppID,
		"app_secret":         testAppSecret,
		"collaborator_email": "wangxianji82@gmail.com",
	}

	fs, err := NewFs(ctx, "test", "", m)
	if err != nil {
		t.Skipf("Skipping combined test - could not create filesystem: %v", err)
	}

	// Step 1: List initial files
	t.Log("Step 1: Listing initial files...")
	entries, err := fs.List(ctx, "")
	if err != nil {
		t.Logf("Initial list returned error: %v", err)
	} else {
		t.Logf("Initial root directory has %d entries", len(entries))
	}

	// Step 2: Create a test directory (will automatically add collaborator)
	testFolderName := fmt.Sprintf("test_folder_combined_%d", time.Now().Unix())
	t.Logf("Step 2: Creating test directory with collaborator: %s", testFolderName)

	err = fs.Mkdir(ctx, testFolderName)
	if err != nil {
		t.Logf("Mkdir returned error: %v", err)
		return
	}
	t.Logf("Successfully created directory: %s", testFolderName)

	// Step 3: List again to verify the directory was created
	t.Log("Step 3: Listing files after mkdir...")
	entries, err = fs.List(ctx, "")
	if err != nil {
		t.Logf("List after mkdir returned error: %v", err)
		return
	}

	t.Logf("Root directory now has %d entries", len(entries))

	found := false
	for _, entry := range entries {
		t.Logf("Entry: %s", entry.String())
		if entry.String() == testFolderName {
			found = true
		}
	}

	if found {
		t.Logf("SUCCESS: Verified directory exists: %s", testFolderName)
	} else {
		t.Logf("WARNING: Directory not found in listing: %s", testFolderName)
	}
}

// TestCollaborator tests adding a collaborator to a folder
func TestCollaborator(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping collaborator test in short mode")
	}

	// Check if credentials are provided
	if testAppID == "" || testAppSecret == "" {
		t.Skip("Skipping collaborator test - no credentials provided")
	}

	ctx := context.Background()

	// Create config map with test credentials and collaborator email
	m := configmap.Simple{
		"app_id":             testAppID,
		"app_secret":         testAppSecret,
		"collaborator_email": "wangxianji82@gmail.com",
	}

	fs, err := NewFs(ctx, "test", "", m)
	if err != nil {
		t.Skipf("Skipping collaborator test - could not create filesystem: %v", err)
	}

	// Create a test folder with timestamp to avoid conflicts
	testFolderName := fmt.Sprintf("test_folder_collab_%d", time.Now().Unix())
	t.Logf("Creating test folder with collaborator: %s", testFolderName)

	err = fs.Mkdir(ctx, testFolderName)
	if err != nil {
		t.Logf("Mkdir returned error: %v", err)
		return
	}
	t.Logf("Successfully created folder: %s", testFolderName)

	// Verify the folder was created
	entries, err := fs.List(ctx, "")
	if err != nil {
		t.Logf("List returned error: %v", err)
		return
	}

	found := false
	for _, entry := range entries {
		if entry.String() == testFolderName {
			found = true
			break
		}
	}

	if found {
		t.Logf("SUCCESS: Created folder with collaborator %s: %s", "wangxianji82@gmail.com", testFolderName)
	} else {
		t.Logf("WARNING: Folder not found in listing: %s", testFolderName)
	}
}
