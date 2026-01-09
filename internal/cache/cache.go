package cache

import (
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var cacheDir = ".cache/rfc-mcp"

var cacheTTL time.Duration

var httpClient = &http.Client{Timeout: 20 * time.Second}

// Meta はキャッシュされたファイルのメタデータを保持します
type Meta struct {
	ETag         string    `json:"etag"`
	LastModified string    `json:"last_modified"`
	CachedAt     time.Time `json:"cached_at"`
}

// SetCacheDir はキャッシュディレクトリを設定します
func SetCacheDir(dir string) {
	cacheDir = dir
}

// SetCacheTTL はキャッシュ再検証のための生存時間を設定します。
// ttl > 0 かつキャッシュされたデータが ttl より新しい場合、FetchWithCache はネットワークにアクセスしません。
func SetCacheTTL(ttl time.Duration) {
	cacheTTL = ttl
}

// EnsureCacheDir はキャッシュディレクトリが存在することを保証します
func EnsureCacheDir() error {
	return os.MkdirAll(cacheDir, 0755)
}

// GetCachePath は指定されたURLのファイルパスを返します
func GetCachePath(url string) string {
	hash := md5.Sum([]byte(url))
	filename := hex.EncodeToString(hash[:]) + ".xml"
	return filepath.Join(cacheDir, filename)
}

// GetMetaPath は指定されたURLのメタデータファイルパスを返します
func GetMetaPath(url string) string {
	hash := md5.Sum([]byte(url))
	filename := hex.EncodeToString(hash[:]) + ".meta.json"
	return filepath.Join(cacheDir, filename)
}

// CleanUpOldCacheFiles は指定された期間より古いキャッシュファイルを削除します
func CleanUpOldCacheFiles(maxAge time.Duration) error {
	// Ensure cache directory exists
	if err := EnsureCacheDir(); err != nil {
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Walk through cache directory
	return filepath.Walk(cacheDir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			// Skip files that can't be accessed
			return nil
		}

		// Skip directories
		if info.IsDir() {
			return nil
		}

		// Check if file is older than maxAge
		if time.Since(info.ModTime()) > maxAge {
			// Delete the file
			if err := os.Remove(path); err != nil {
				// Log error but continue cleaning up other files
				fmt.Printf("Warning: Failed to remove old cache file %s: %v\n", path, err)
			} else {
				fmt.Printf("Removed old cache file: %s\n", path)
			}
		}

		return nil
	})
}

// FetchWithCache はETagとLast-Modifiedを使用してURLを取得します
func FetchWithCache(url string) ([]byte, error) {
	return FetchWithCacheCtx(context.Background(), url)
}

// FetchWithCacheCtx はETag/Last-Modifiedを使用してURLを取得します。
// また、cacheTTLも考慮されます：キャッシュされたデータが新しければ、ネットワークリクエストなしで返されます。
func FetchWithCacheCtx(ctx context.Context, url string) ([]byte, error) {
	// Ensure cache directory exists
	if err := EnsureCacheDir(); err != nil {
		return nil, fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Check if we have cached data
	cachePath := GetCachePath(url)
	metaPath := GetMetaPath(url)

	// Read metadata if exists
	var meta Meta
	if metaBytes, err := os.ReadFile(metaPath); err == nil {
		if err := json.Unmarshal(metaBytes, &meta); err != nil {
			// メタデータを解析できない場合は、再ダウンロードします
			meta = Meta{}
		}
	}

	// キャッシュが十分新しければ、ネットワークアクセスを回避します。
	if cacheTTL > 0 && !meta.CachedAt.IsZero() {
		if time.Since(meta.CachedAt) < cacheTTL {
			if data, err := os.ReadFile(cachePath); err == nil {
				return data, nil
			}
			// キャッシュの読み取りに失敗した場合は、再ダウンロードに移行します。
		}
	}

	// Create HTTP request
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// あればキャッシュヘッダーを設定
	if meta.ETag != "" {
		req.Header.Set("If-None-Match", meta.ETag)
	}
	if meta.LastModified != "" {
		req.Header.Set("If-Modified-Since", meta.LastModified)
	}

	// Make the request
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch URL: %w", err)
	}
	defer resp.Body.Close()

	// 304 Not Modified を処理
	if resp.StatusCode == http.StatusNotModified {
		// Return cached data
		data, err := os.ReadFile(cachePath)
		if err != nil {
			return nil, fmt.Errorf("キャッシュデータの読み取りに失敗しました: %w", err)
		}
		return data, nil
	}

	// その他の2xx以外のレスポンスを処理
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP error %d", resp.StatusCode)
	}

	// レスポンスボディを読み取る
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// キャッシュに保存
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		return nil, fmt.Errorf("failed to save cache: %w", err)
	}

	// メタデータを更新
	newMeta := Meta{
		ETag:         resp.Header.Get("ETag"),
		LastModified: resp.Header.Get("Last-Modified"),
		CachedAt:     time.Now(),
	}

	metaBytes, err := json.Marshal(newMeta)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal metadata: %w", err)
	}

	if err := os.WriteFile(metaPath, metaBytes, 0644); err != nil {
		return nil, fmt.Errorf("failed to save metadata: %w", err)
	}

	return data, nil
}
