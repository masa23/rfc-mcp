package search

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/xml"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"github.com/masa23/rfc-mcp/internal/cache"
)

const (
	rfcIndexURL = "https://www.rfc-editor.org/rfc-index.xml"
	rfcBaseURL  = "https://www.rfc-editor.org/rfc/rfc%d.xml"
)

// RFCXML はXML形式のRFCドキュメントのルート要素を表します
type RFCXML struct {
	XMLName  xml.Name  `xml:"rfc"`
	Title    string    `xml:"front>title"`
	Sections []Section `xml:"middle>section"`
}

// Section はRFCドキュメント内のセクションを表します
type Section struct {
	Title    string    `xml:"title,attr"`
	Number   string    `xml:"number,attr"`
	Content  string    `xml:",innerxml"`
	Sections []Section `xml:"section"`
}

var (
	indexMu      sync.Mutex
	indexItems   []RFCItem
	indexDataMD5 string
)

// Search は指定されたクエリとリミットでRFC検索を実行します
func Search(query string, limit int) ([]RFCItem, error) {
	// キャッシュを使用してXMLデータを取得
	data, err := cache.FetchWithCache(rfcIndexURL)
	if err != nil {
		return nil, err
	}

	// 変更があった場合のみXMLデータを解析します。
	items, err := getOrParseIndexItems(data)
	if err != nil {
		return nil, err
	}

	// Apply search logic
	results := SearchRFCs(items, strings.ToLower(query), limit)

	return results, nil
}

// ImprovedSearch は改善された検索アルゴリズムを使用してRFCを検索します
func ImprovedSearch(query string, limit int) ([]RFCItem, error) {
	// キャッシュを使用してXMLデータを取得
	data, err := cache.FetchWithCache(rfcIndexURL)
	if err != nil {
		return nil, err
	}

	// 変更があった場合のみXMLデータを解析します。
	items, err := getOrParseIndexItems(data)
	if err != nil {
		return nil, err
	}

	// クエリを解析
	keywords := AnalyzeQuery(query)

	// Apply improved search logic with scoring
	results := ScoredSearchRFCs(items, keywords, limit)

	return results, nil
}

// FilteredSearch はフィルタリングオプションを使用してRFCを検索します
func FilteredSearch(query string, limit int, filters map[string]string) ([]RFCItem, error) {
	// キャッシュを使用してXMLデータを取得
	data, err := cache.FetchWithCache(rfcIndexURL)
	if err != nil {
		return nil, err
	}

	// 変更があった場合のみXMLデータを解析します。
	items, err := getOrParseIndexItems(data)
	if err != nil {
		return nil, err
	}

	// クエリを解析
	keywords := AnalyzeQuery(query)

	// Apply improved search logic with scoring
	results := ScoredSearchRFCs(items, keywords, limit)

	// フィルタリングを適用
	filteredResults := applyFilters(results, filters)

	return filteredResults, nil
}

// applyFilters は指定されたフィルタを検索結果に適用します
func applyFilters(items []RFCItem, filters map[string]string) []RFCItem {
	var filtered []RFCItem

	for _, item := range items {
		match := true

		// ステータスフィルターを適用
		if status, ok := filters["status"]; ok && status != "" {
			if item.Status != status {
				match = false
			}
		}

		// 日付フィルターを適用（年）
		if year, ok := filters["year"]; ok && year != "" {
			if !strings.Contains(item.Date, year) {
				match = false
			}
		}

		if match {
			filtered = append(filtered, item)
		}
	}

	return filtered
}

// FuzzySearch はファジー検索を使用してRFCを検索します
func FuzzySearch(query string, limit int) ([]RFCItem, error) {
	// キャッシュを使用してXMLデータを取得
	data, err := cache.FetchWithCache(rfcIndexURL)
	if err != nil {
		return nil, err
	}

	// 変更があった場合のみXMLデータを解析します。
	items, err := getOrParseIndexItems(data)
	if err != nil {
		return nil, err
	}

	// Apply fuzzy search logic
	results := FuzzySearchRFCs(items, strings.ToLower(query), limit)

	return results, nil
}

// AnalyzeQuery は検索クエリを解析し、重要なキーワードを抽出します
func AnalyzeQuery(query string) []string {
	// クエリを小文字に変換
	lowerQuery := strings.ToLower(query)

	// ストップワードを定義
	stopWords := map[string]bool{
		"the": true, "and": true, "or": true, "but": true, "in": true, "on": true, "at": true, "to": true,
		"for": true, "of": true, "with": true, "by": true, "a": true, "an": true, "is": true, "are": true,
		"was": true, "were": true, "be": true, "been": true, "have": true, "has": true, "had": true,
		"do": true, "does": true, "did": true, "will": true, "would": true, "could": true, "should": true,
	}

	// キーワードを抽出
	words := strings.Fields(lowerQuery)
	var keywords []string

	for _, word := range words {
		// 句読点や特殊文字を削除
		cleanWord := strings.Trim(word, ".,!?;:\"'()[]{}")

		// ストップワードでない場合にキーワードとして追加
		if !stopWords[cleanWord] && len(cleanWord) > 0 {
			keywords = append(keywords, cleanWord)
		}
	}

	return keywords
}

func getOrParseIndexItems(data []byte) ([]RFCItem, error) {
	h := md5.Sum(data)
	md5s := hex.EncodeToString(h[:])

	indexMu.Lock()
	defer indexMu.Unlock()

	if indexDataMD5 == md5s && len(indexItems) > 0 {
		return indexItems, nil
	}

	items, err := ParseRFCIndex(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}

	indexItems = items
	indexDataMD5 = md5s
	return indexItems, nil
}

// FetchRFC はRFCドキュメントのコンテンツを取得します
func FetchRFC(number int, maxBytes int) (string, error) {
	// 利用可能な場合はXML形式を優先します。
	xmlURL := fmt.Sprintf(rfcBaseURL, number)
	data, err := cache.FetchWithCache(xmlURL)
	if err == nil {
		// これが完全なRFC XML ("<rfc ...>") の場合、解析してプレーンテキストとしてレンダリングします。
		// 一部のRFC番号は参照XML ("<reference ...>") のみを持っているため、その場合は
		// 読みやすさのためにテキストRFCにフォールバックします。
		s := string(data)
		if strings.Contains(s, "<rfc") {
			var rfc RFCXML
			if uerr := xml.Unmarshal(data, &rfc); uerr == nil {
				rendered := renderRFCPlainText(&rfc)
				if maxBytes > 0 && len(rendered) > maxBytes {
					rendered = rendered[:maxBytes]
				}
				return rendered, nil
			}
			// XMLの解析に失敗した場合は、以下で生のXMLを返すことにフォールバックします。
		} else if strings.Contains(s, "<reference") {
			// reference XML isn't the RFC body; fall back to .txt
			data = nil
		} else {
			// 不明なXML風のコンテンツ；そのまま返します。
			if maxBytes > 0 && len(s) > maxBytes {
				s = s[:maxBytes]
			}
			return s, nil
		}

		// まだXMLバイトがある場合（解析失敗）、サイズ制限付きで生のXMLを返します。
		if data != nil {
			if maxBytes > 0 && len(data) > maxBytes {
				data = data[:maxBytes]
			}
			return string(data), nil
		}
	}

	// XMLが利用できない場合はテキスト形式にフォールバックします。
	textURL := fmt.Sprintf("https://www.rfc-editor.org/rfc/rfc%d.txt", number)
	data, err = cache.FetchWithCache(textURL)
	if err != nil {
		return "", err
	}

	s := string(data)
	if maxBytes > 0 && len(s) > maxBytes {
		s = s[:maxBytes]
	}
	return s, nil
}

// ExtractRFC はRFCドキュメントからセクションを抽出します
func ExtractRFC(number int, section string) (string, error) {
	// RFCコンテンツを取得
	content, err := FetchRFC(number, 0) // No size limit for extraction
	if err != nil {
		return "", fmt.Errorf("failed to fetch RFC %d: %w", number, err)
	}

	sec := normalizeSection(section)
	if sec == "" {
		return "", fmt.Errorf("invalid section: %q", section)
	}

	// コンテンツがXML形式かどうかを確認
	if strings.Contains(content, "<reference") {
		// これは参照XMLであり、完全なRFC XMLではありません
		// テキストベースの抽出にフォールバックします
		return extractSectionFromText(content, sec)
	} else if strings.Contains(content, "<rfc") {
		// これは完全なRFC XMLです
		// XMLコンテンツを解析します
		var rfc RFCXML
		if err := xml.Unmarshal([]byte(content), &rfc); err != nil {
			return "", fmt.Errorf("failed to parse RFC XML: %w", err)
		}

		// 指定されたセクションを探す
		sectionContent := findSection(rfc.Sections, sec)
		if sectionContent == "" {
			return "", fmt.Errorf("section %s not found", section)
		}

		return sectionContent, nil
	} else {
		// テキスト形式を想定
		return extractSectionFromText(content, sec)
	}
}

// extractSectionFromText はテキストベースのRFCコンテンツからセクションを抽出します
func extractSectionFromText(content, section string) (string, error) {
	lines := strings.Split(content, "\n")
	var extracted []string

	// "3.  Values" や "3. Values" のようなセクション見出しに一致させます
	// セクション番号の後にはピリオド、任意のスペース、そしてタイトルが続きます
	// "3. Values" と "3.  Values" （複数のスペース）の両方に一致させる必要があります
	// また、目次エントリ（ページ番号を持つ）と実際のセクション見出しを区別する必要があります
	// 実際のセクション見出しは末尾にページ番号を持ちません
	// 目次エントリは "3.  Values  . . . . . . . . . . . . . . . . . . . . . . . . . . .   6" のようなパターンを持ちます
	// 実際のセクション見出しは "3.  Values" のようなパターンを持ちます
	// 違いは、目次エントリが末尾にドットとページ番号を持っていることです
	// 末尾にドットと数字（ページ番号）が続く目次エントリを除外する正規表現
	startRe := regexp.MustCompile(fmt.Sprintf(`^\s*%s\.\s+[^.]+\s*$`, regexp.QuoteMeta(section)))
	// Generic numeric heading like "3.1. Title" or "3. Title" or "4. Objects"
	headingRe := regexp.MustCompile(`^\s*(\d+(?:\.\d+)*)\.`)

	inSection := false

	for _, line := range lines {
		// Check if this is the start of the desired section
		if !inSection {
			if startRe.MatchString(line) {
				inSection = true
				extracted = append(extracted, line)
			}
			continue
		}

		// If we're in the section, check if this line starts a new section
		// If it does, we've reached the end of the desired section
		if headingRe.MatchString(line) {
			// This is a new section header, so we're done
			break
		}

		extracted = append(extracted, line)
	}

	if len(extracted) == 0 {
		return "", fmt.Errorf("section %s not found", section)
	}

	// Trim trailing empty lines
	for len(extracted) > 0 && strings.TrimSpace(extracted[len(extracted)-1]) == "" {
		extracted = extracted[:len(extracted)-1]
	}

	return strings.Join(extracted, "\n"), nil
}

// findSection は指定された番号のセクションを再帰的に検索します
func findSection(sections []Section, sectionNumber string) string {
	for _, section := range sections {
		// 完全一致をチェック
		if section.Number == sectionNumber {
			// セクションからテキストコンテンツを抽出
			return extractTextContent(section.Content)
		}

		// 前方一致をチェック（例：セクション"3"が要求されたときに"3.1"なども検索）
		if strings.HasPrefix(section.Number, sectionNumber+".") {
			// セクションからテキストコンテンツを抽出
			return extractTextContent(section.Content)
		}

		// サブセクション内で再帰的に検索
		if content := findSection(section.Sections, sectionNumber); content != "" {
			return content
		}
	}
	return ""
}

// extractTextContent はXMLセクションコンテンツからテキストコンテンツを抽出します
func extractTextContent(xmlContent string) string {
	// XMLタグを削除してプレーンテキストを返す
	re := regexp.MustCompile(`<[^>]*>`)
	text := re.ReplaceAllString(xmlContent, "")

	// 余分な空白をクリーンアップ
	text = strings.TrimSpace(text)
	text = regexp.MustCompile(`\n\s*\n`).ReplaceAllString(text, "\n\n")

	return text
}

var reSectionNumber = regexp.MustCompile(`\d+(?:\.\d+)*`)

// normalizeSection は "Section 3.1" / "3.1" / "3.1." のような入力を受け入れ、"3.1" を返します。
func normalizeSection(in string) string {
	in = strings.TrimSpace(in)
	if in == "" {
		return ""
	}
	if m := reSectionNumber.FindString(in); m != "" {
		// 末尾のドットがあればトリム ("3.1." -> "3.1")
		return strings.TrimSuffix(m, ".")
	}
	// 一部のRFCでは付録ラベルを使用しますが、まだサポートしていません。
	return ""
}

// renderRFCPlainText は解析されたRFC XMLドキュメントを読みやすいプレーンテキスト形式に変換します。
func renderRFCPlainText(rfc *RFCXML) string {
	var b strings.Builder

	title := strings.TrimSpace(rfc.Title)
	if title != "" {
		b.WriteString(title)
		b.WriteString("\n\n")
	}

	for _, s := range rfc.Sections {
		renderSectionPlainText(&b, s, 0)
	}

	return b.String()
}

func renderSectionPlainText(b *strings.Builder, s Section, depth int) {
	heading := strings.TrimSpace(strings.TrimSpace(s.Number) + " " + strings.TrimSpace(s.Title))
	if heading != "" {
		b.WriteString(heading)
		b.WriteString("\n")
	}

	// このセクションの内部XMLからネストされた<section>ブロックを削除して、サブセクションのテキストが重複しないようにします。
	bodyXML := stripNestedSectionsXML(s.Content)
	body := extractTextContent(bodyXML)
	if body != "" {
		b.WriteString(body)
		b.WriteString("\n\n")
	} else {
		b.WriteString("\n")
	}

	for _, sub := range s.Sections {
		renderSectionPlainText(b, sub, depth+1)
	}
}

var reNestedSection = regexp.MustCompile(`(?s)<section\b[^>]*>.*?</section>`)

func stripNestedSectionsXML(in string) string {
	// ネストされた<section>ブロックを削除
	return reNestedSection.ReplaceAllString(in, "")
}

// FuzzySearchRFCs は編集距離アルゴリズムを使用してRFCアイテムをファジー検索します
func FuzzySearchRFCs(items []RFCItem, query string, limit int) []RFCItem {
	type scoredItem struct {
		item  RFCItem
		score int
	}

	var scored []scoredItem

	for _, item := range items {
		// タイトルと番号でスコアを計算
		titleScore := levenshteinDistance(strings.ToLower(item.Title), query)
		numberStr := fmt.Sprintf("%d", item.Number)

		// 数字の完全一致に高いスコアを与える
		var numberScore int
		if strings.Contains(numberStr, query) {
			numberScore = 0 // 完全に含まれている場合は最高スコア
		} else {
			numberScore = levenshteinDistance(numberStr, query)
		}

		// 最も良いスコアを使用（低いほど良い）
		bestScore := titleScore
		if numberScore < bestScore {
			bestScore = numberScore
		}

		// スコアが閾値以下の項目のみを結果に含める
		if bestScore <= 3 { // 閾値は調整可能
			scored = append(scored, scoredItem{item: item, score: bestScore})
		}
	}

	// スコアでソート（低いスコアがより良い一致）
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score < scored[j].score
	})

	// リミットに応じて結果をスライス
	resultLimit := limit
	if resultLimit <= 0 || resultLimit > len(scored) {
		resultLimit = len(scored)
	}

	results := make([]RFCItem, resultLimit)
	for i := 0; i < resultLimit; i++ {
		results[i] = scored[i].item
	}

	return results
}

// levenshteinDistance は2つの文字列間の編集距離を計算します
func levenshteinDistance(s1, s2 string) int {
	m := len(s1)
	n := len(s2)

	// dp配列を初期化
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	// 基底ケース
	for i := 0; i <= m; i++ {
		dp[i][0] = i
	}
	for j := 0; j <= n; j++ {
		dp[0][j] = j
	}

	// dpテーブルを埋める
	for i := 1; i <= m; i++ {
		for j := 1; j <= n; j++ {
			if s1[i-1] == s2[j-1] {
				dp[i][j] = dp[i-1][j-1]
			} else {
				dp[i][j] = min(dp[i-1][j], dp[i][j-1], dp[i-1][j-1]) + 1
			}
		}
	}

	return dp[m][n]
}

// min は3つの整数の最小値を返します
func min(a, b, c int) int {
	if a <= b && a <= c {
		return a
	}
	if b <= a && b <= c {
		return b
	}
	return c
}

// ScoredSearchRFCs はスコアリングアルゴリズムを使用してRFCアイテムを検索します
func ScoredSearchRFCs(items []RFCItem, keywords []string, limit int) []RFCItem {
	type scoredItem struct {
		item  RFCItem
		score float64
	}

	var scored []scoredItem

	for _, item := range items {
		score := calculateScore(item, keywords)
		if score > 0 {
			scored = append(scored, scoredItem{item: item, score: score})
		}
	}

	// スコアでソート（高いスコアがより良い一致）
	sort.Slice(scored, func(i, j int) bool {
		return scored[i].score > scored[j].score
	})

	// リミットに応じて結果をスライス
	resultLimit := limit
	if resultLimit <= 0 || resultLimit > len(scored) {
		resultLimit = len(scored)
	}

	results := make([]RFCItem, resultLimit)
	for i := 0; i < resultLimit; i++ {
		results[i] = scored[i].item
	}

	return results
}

// calculateScore はRFCアイテムとキーワードに基づいてスコアを計算します
func calculateScore(item RFCItem, keywords []string) float64 {
	var score float64
	title := strings.ToLower(item.Title)

	for _, keyword := range keywords {
		// タイトルにキーワードが含まれている場合にスコアを加算
		if strings.Contains(title, keyword) {
			// 完全一致の場合により高いスコアを付与
			if title == keyword {
				score += 10
			} else {
				score += 5
			}
		}

		// RFC番号がキーワードと一致する場合にスコアを加算
		numberStr := fmt.Sprintf("%d", item.Number)
		if strings.Contains(numberStr, keyword) {
			score += 3
		}
	}

	return score
}
