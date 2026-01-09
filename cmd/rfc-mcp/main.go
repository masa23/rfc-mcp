package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/masa23/rfc-mcp/config"
	"github.com/masa23/rfc-mcp/internal/cache"
	"github.com/masa23/rfc-mcp/internal/search"

	"github.com/google/jsonschema-go/jsonschema"
	"github.com/modelcontextprotocol/go-sdk/mcp"
)

func main() {
	var mode string
	var httpAddr string
	var httpPath string
	var confPath string

	flag.StringVar(&mode, "mode", "", "stdio or http")
	flag.StringVar(&httpAddr, "http-addr", "", "http listen address")
	flag.StringVar(&httpPath, "http-path", "", "http mcp endpoint path")

	// 環境変数から設定ファイルパスを取得、なければデフォルト値を使用
	defaultConfPath := "config.yaml"
	if envConfPath := os.Getenv("RFC_MCP_CONFIG"); envConfPath != "" {
		defaultConfPath = envConfPath
	}
	flag.StringVar(&confPath, "config", defaultConfPath, "path to config file")
	flag.Parse()

	// 設定を読み込む
	conf, err := config.Load(confPath)
	if err != nil {
		log.Printf("Warning: Could not load config from %s: %v", confPath, err)
		conf = config.Default()
	}

	// Ensure defaults are applied even when config file is partial.
	conf.ApplyDefaults()

	if mode == string(config.ModeStdio) || mode == string(config.ModeHTTP) {
		conf.Server.Mode = config.ModeType(mode)
	}

	// conf を使用して設定に依存する操作を行う
	if httpPath != "" {
		conf.Server.Path = httpPath
	}

	if httpAddr == "" {
		httpAddr = fmt.Sprintf("%s:%d", conf.Server.Host, conf.Server.Port)
	}

	if conf.Cache.TTL != "" {
		if d, err := time.ParseDuration(conf.Cache.TTL); err != nil {
			log.Printf("Warning: invalid cache.ttl %q: %v", conf.Cache.TTL, err)
			log.Printf("Using default cache TTL of 1 hour")
			cache.SetCacheTTL(1 * time.Hour)
		} else {
			cache.SetCacheTTL(d)
		}
	} else {
		// デフォルトのキャッシュTTLは1時間
		cache.SetCacheTTL(1 * time.Hour)
	}

	// Cache settings
	if conf.Cache.Dir != "" {
		cache.SetCacheDir(conf.Cache.Dir)
		if err := cache.EnsureCacheDir(); err != nil {
			log.Printf("Warning: could not create cache dir %q: %v", conf.Cache.Dir, err)
		}
	}

	// 古いキャッシュファイルを定期的にクリーンアップ
	go func() {
		// 24時間ごとにクリーンアップを実行
		ticker := time.NewTicker(24 * time.Hour)
		defer ticker.Stop()

		// 初回実行
		if err := cache.CleanUpOldCacheFiles(7 * 24 * time.Hour); err != nil {
			log.Printf("Warning: Failed to clean up old cache files: %v", err)
		}

		for range ticker.C {
			if err := cache.CleanUpOldCacheFiles(7 * 24 * time.Hour); err != nil {
				log.Printf("Warning: Failed to clean up old cache files: %v", err)
			}
		}
	}()

	// 新しいMCPサーバーを作成
	impl := &mcp.Implementation{
		Name:    "rfc-mcp-server",
		Version: "0.1.0",
	}

	s := mcp.NewServer(impl, nil)

	// rfc_searchツールを追加
	mcp.AddTool[search.SearchInput, search.SearchResults](s, &mcp.Tool{
		Name:        "rfc_search",
		Description: "Search RFCs using rfc-index.xml",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"query": {
					Type:        "string",
					Description: "Search query keywords",
				},
				"limit": {
					Type:        "integer",
					Description: "Maximum number of results to return",
				},
			},
			Required: []string{"query"},
		},
	}, handleRFCSearch)

	// rfc_fetchツールを追加
	mcp.AddTool[search.FetchInput, search.FetchResult](s, &mcp.Tool{
		Name:        "rfc_fetch",
		Description: "Fetch the content of an RFC document",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"number": {
					Type:        "integer",
					Description: "RFC number to fetch",
				},
				"maxBytes": {
					Type:        "integer",
					Description: "Maximum number of bytes to return",
				},
			},
			Required: []string{"number"},
		},
	}, handleRFCFetch)

	// rfc_extractツールを追加
	mcp.AddTool[search.ExtractInput, search.ExtractResult](s, &mcp.Tool{
		Name:        "rfc_extract",
		Description: "Extract a section from an RFC document",
		InputSchema: &jsonschema.Schema{
			Type: "object",
			Properties: map[string]*jsonschema.Schema{
				"number": {
					Type:        "integer",
					Description: "RFC number to extract from",
				},
				"section": {
					Type:        "string",
					Description: "Section to extract (e.g., 'Section 3.1')",
				},
			},
			Required: []string{"number", "section"},
		},
	}, handleRFCExtract)

	// 注: ここにツール/リソースの登録を行う

	switch conf.Server.Mode {
	case config.ModeStdio:
		log.Printf("Starting MCP server in stdio mode")
		if err := s.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
			log.Fatalf("Failed to start MCP server in stdio mode: %v", err)
		}

	case config.ModeHTTP:
		log.Printf("Starting MCP server in http mode on %s%s", httpAddr, conf.Server.Path)

		// Streamable HTTP (SSE) handler
		handler := mcp.NewStreamableHTTPHandler(func(_ *http.Request) *mcp.Server {
			return s
		}, nil)

		mux := http.NewServeMux()
		mux.Handle(conf.Server.Path, loggingMiddleware(handler))

		// optional health check
		mux.HandleFunc("/healthz", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusOK)
			_, _ = w.Write([]byte("ok"))
		})

		srv := &http.Server{
			Addr:              httpAddr,
			Handler:           mux,
			ReadHeaderTimeout: 5 * time.Second,
		}

		if err := srv.ListenAndServe(); err != nil {
			if !errors.Is(err, http.ErrServerClosed) {
				log.Fatal(err)
			}
		}

	default:
		log.Fatalf("Unknown mode: %s (use stdio or http)", mode)
	}
}

// handleRFCSearch は rfc_search ツールを実装します
func handleRFCSearch(ctx context.Context, request *mcp.CallToolRequest, input search.SearchInput) (*mcp.CallToolResult, search.SearchResults, error) {
	// 指定されていない場合はデフォルトリミットを設定
	if input.Limit <= 0 {
		input.Limit = 10
	}

	// 検索を実行
	items, err := search.Search(input.Query, input.Limit)
	if err != nil {
		return nil, search.SearchResults{}, err
	}

	results := search.SearchResults{
		Items: items,
	}

	return nil, results, nil
}

// handleRFCFetch は rfc_fetch ツールを実装します
func handleRFCFetch(ctx context.Context, request *mcp.CallToolRequest, input search.FetchInput) (*mcp.CallToolResult, search.FetchResult, error) {
	// 指定されていない場合はデフォルトのmaxBytesを設定
	if input.MaxBytes <= 0 {
		input.MaxBytes = 10000 // 10KB default
	}

	// RFCコンテンツを取得
	content, err := search.FetchRFC(input.Number, input.MaxBytes)
	if err != nil {
		return nil, search.FetchResult{}, err
	}

	result := search.FetchResult{
		Content: content,
	}

	return nil, result, nil
}

// handleRFCExtract は rfc_extract ツールを実装します
func handleRFCExtract(ctx context.Context, request *mcp.CallToolRequest, input search.ExtractInput) (*mcp.CallToolResult, search.ExtractResult, error) {
	// RFCからセクションを抽出
	content, err := search.ExtractRFC(input.Number, input.Section)
	if err != nil {
		return nil, search.ExtractResult{}, err
	}

	result := search.ExtractResult{
		Content: content,
	}

	return nil, result, nil
}

// loggingMiddleware はHTTPリクエストのアクセスログを標準エラー出力に出力するミドルウェアです。
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		// ResponseWriterをラップしてステータスコードを取得できるようにする
		wrapper := &responseWriterWrapper{ResponseWriter: w, statusCode: http.StatusOK}

		// 次のハンドラーを呼び出す
		next.ServeHTTP(wrapper, r)

		// アクセスログを出力
		log.Printf(
			"%s %s %s %d %v",
			time.Now().Format("2006/01/02 15:04:05"),
			r.Method,
			r.URL.Path,
			wrapper.statusCode,
			time.Since(start),
		)
	})
}

// responseWriterWrapper はステータスコードを取得できるようにするResponseWriterのラッパーです。
type responseWriterWrapper struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriterWrapper) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
