package main

import (
	"bufio"
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"golang.org/x/net/html"
)

const (
	GROQ_API_URL = "https://api.groq.com/openai/v1/chat/completions"
	GROQ_MODEL   = "llama-3.1-8b-instant"
)

var (
	//go:embed system-prompt.md
	systemPrompt string

	//go:embed article-template.md
	articleTemplate string
)

func main() {
	if len(os.Args) != 3 {
		fmt.Println("Usage: report <output-folder> <url>")
		os.Exit(1)
	}

	outputFolder := os.Args[1]
	articleUrl := os.Args[2]

	article, err := scrapeArticle(articleUrl)
	if err != nil {
		fmt.Printf("Error: %+v\n", err)
		os.Exit(1)
	}
	if !isValidWindowsFilename(article.Title) {
		fmt.Printf("Article title '%s' is not a valid Windows filename\n", article.Title)
		article.Title = getUserInputtedArticleTitle()
	}

	groqApiKey := os.Getenv("GROQ_API_KEY")
	if groqApiKey == "" {
		fmt.Println("Error: GROQ_API_KEY environment variable not set")
		os.Exit(1)
	}

	articleSummary, err := getArticleSummary(article, systemPrompt, groqApiKey)
	if err != nil {
		fmt.Printf("Error: %+v\n", err)
		os.Exit(1)
	}

	article.Summary = &articleSummary

	err = exportArticle(outputFolder, article)
	if err != nil {
		fmt.Printf("Error: %+v\n", err)
		os.Exit(1)
	}
}

type Article struct {
	Url     string
	Title   string
	Content string
	Summary *ArticleSummary
}

func scrapeArticle(articleUrl string) (Article, error) {
	page, err := fetchUrlAndReturnPage(articleUrl)
	if err != nil {
		return Article{}, fmt.Errorf("getting page at '%s': %w", articleUrl, err)
	}

	title, err := scrapeArticleTitle(page)
	if err != nil {
		return Article{}, fmt.Errorf("scraping article title: %w", err)
	}

	content, err := scrapePageBody(page)
	if err != nil {
		return Article{}, fmt.Errorf("scraping page body: %w", err)
	}

	return Article{
		Url:     articleUrl,
		Title:   title,
		Content: content,
	}, nil
}

func fetchUrlAndReturnPage(url string) (string, error) {
	res, err := http.Get(url)
	if err != nil {
		return "", fmt.Errorf("fetching url '%s': %w", url, err)
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return "", fmt.Errorf("reading body for url '%s': %w", url, err)
	}

	return string(body), nil
}

func scrapeArticleTitle(pageContent string) (string, error) {
	h1Regex := `<h1.*?>(.*?)</h1>`
	h1Match := findFirstMatch(h1Regex, pageContent)
	if h1Match == "" {
		return "", fmt.Errorf("no h1 found in page content")
	}

	return h1Match, nil
}

func scrapePageBody(pageContent string) (string, error) {
	bodyRegex := `(?s)<body.*?>(.*?)</body>`
	bodyMatch := findFirstMatch(bodyRegex, pageContent)
	if bodyMatch == "" {
		return "", fmt.Errorf("no body found in page content")
	}

	return cleanBodyContent(bodyMatch), nil
}

func findFirstMatch(regex string, content string) string {
	re := regexp.MustCompile(regex)
	match := re.FindStringSubmatch(content)
	if len(match) == 0 {
		return ""
	} else {
		return match[1]
	}
}

func cleanBodyContent(content string) string {
	doc, err := html.Parse(strings.NewReader(content))
	if err != nil {
		fmt.Println("could not parse content as HTML: %w", err)
		return content
	}

	var buf bytes.Buffer

	var traverse func(*html.Node)
	traverse = func(n *html.Node) {
		if n.Type == html.TextNode {
			buf.WriteString(n.Data)
		} else if n.Type == html.ElementNode {
			switch n.Data {
			case "script", "style", "nav", "footer", "header":
				return
			case "p", "h1", "h2", "h3", "h4", "h5", "h6", "div":
				buf.WriteString("\n")
			case "img":
				for _, attr := range n.Attr {
					if attr.Key == "alt" {
						buf.WriteString("[Image: " + attr.Val + "]")
						break
					}
				}
				return
			}
		}

		for c := n.FirstChild; c != nil; c = c.NextSibling {
			traverse(c)
		}

		if n.Type == html.ElementNode {
			switch n.Data {
			case "p", "br", "div":
				buf.WriteString("\n")
			}
		}
	}

	traverse(doc)

	cleaned := strings.Join(strings.Fields(buf.String()), " ")
	return cleaned
}

type GroqMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type GroqRequestBody struct {
	Messages       []GroqMessage `json:"messages"`
	Model          string        `json:"model"`
	Temperature    float64       `json:"temperature"`
	MaxTokens      int           `json:"max_tokens"`
	TopP           float64       `json:"top_p"`
	Stream         bool          `json:"stream"`
	ResponseFormat struct {
		Type string `json:"type"`
	} `json:"response_format"`
	Stop interface{} `json:"stop"`
}

type GroqResponse struct {
	ID      string `json:"id"`
	Object  string `json:"object"`
	Created int64  `json:"created"`
	Model   string `json:"model"`
	Choices []struct {
		Index   int `json:"index"`
		Message struct {
			Role    string `json:"role"`
			Content string `json:"content"`
		} `json:"message"`
		LogProbs     interface{} `json:"logprobs"`
		FinishReason string      `json:"finish_reason"`
	} `json:"choices"`
	Usage struct {
		QueueTime        float64 `json:"queue_time"`
		PromptTokens     int     `json:"prompt_tokens"`
		PromptTime       float64 `json:"prompt_time"`
		CompletionTokens int     `json:"completion_tokens"`
		CompletionTime   float64 `json:"completion_time"`
		TotalTokens      int     `json:"total_tokens"`
		TotalTime        float64 `json:"total_time"`
	} `json:"usage"`
	SystemFingerprint string `json:"system_fingerprint"`
	XGroq             struct {
		ID string `json:"id"`
	} `json:"x_groq"`
}

type GroqErrorResponse struct {
	Error struct {
		Message          string `json:"message"`
		Type             string `json:"type"`
		Code             string `json:"code"`
		FailedGeneration string `json:"failed_generation"`
	} `json:"error"`
}

type ArticleSummary struct {
	Summary   string   `json:"summary"`
	Keypoints []string `json:"keypoints"`
	Tags      []string `json:"tags"`
}

func getArticleSummary(article Article, systemPrompt, groqApiKey string) (ArticleSummary, error) {
	requestBody := GroqRequestBody{
		Messages: []GroqMessage{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: article.Content},
		},
		Model:       GROQ_MODEL,
		Temperature: 1,
		MaxTokens:   1024,
		TopP:        1,
		Stream:      false,
		ResponseFormat: struct {
			Type string `json:"type"`
		}{
			Type: "json_object",
		},
		Stop: nil,
	}

	jsonData, err := json.Marshal(requestBody)
	if err != nil {
		return ArticleSummary{}, fmt.Errorf("marshaling JSON: %w", err)
	}

	req, err := http.NewRequest("POST", GROQ_API_URL, bytes.NewBuffer(jsonData))
	if err != nil {
		return ArticleSummary{}, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+groqApiKey)

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return ArticleSummary{}, fmt.Errorf("sending request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return ArticleSummary{}, fmt.Errorf("reading response body: %w", err)
	}

	var errorResp GroqErrorResponse
	if err := json.Unmarshal(body, &errorResp); err == nil && errorResp.Error.Message != "" {
		return ArticleSummary{}, fmt.Errorf("API error: %s (Type: %s, Code: %s, Failed Generation: %s)",
			errorResp.Error.Message,
			errorResp.Error.Type,
			errorResp.Error.Code,
			errorResp.Error.FailedGeneration)
	}

	var groqResp GroqResponse
	if err := json.Unmarshal(body, &groqResp); err != nil {
		return ArticleSummary{}, fmt.Errorf("unmarshaling response: %w", err)
	}

	if len(groqResp.Choices) == 0 {
		return ArticleSummary{}, fmt.Errorf("no choices in response")
	}

	var articleSummary ArticleSummary
	if err := json.Unmarshal([]byte(groqResp.Choices[0].Message.Content), &articleSummary); err != nil {
		return ArticleSummary{}, fmt.Errorf("unmarshaling article summary: %w", err)
	}

	return articleSummary, nil
}

func exportArticle(outputFolder string, article Article) error {
	if article.Title == "" || article.Summary == nil || len(article.Summary.Keypoints) == 0 || len(article.Summary.Tags) == 0 {
		incompleteArticleStr := fmt.Sprintf(`
		- title: %s (needs to be set)
		- is summary nil: %t (needs to be true)
		- keypoints length: %d (needs > 0)
		- tags length: %d (needs > 0)
		`,
			article.Title,
			article.Summary == nil,
			len(article.Summary.Keypoints),
			len(article.Summary.Tags),
		)
		return fmt.Errorf("article is incomplete: \n%s", incompleteArticleStr)
	}

	currentDate := time.Now().Format("2006-01-02")
	content := string(articleTemplate)
	content = strings.ReplaceAll(content, "KEY_ARTICLE_TITLE", article.Title)
	content = strings.ReplaceAll(content, "KEY_URL", article.Url)
	content = strings.ReplaceAll(content, "KEY_CREATION_DATE", currentDate)
	content = strings.ReplaceAll(content, "KEY_SUMMARY", article.Summary.Summary)
	content = strings.ReplaceAll(content, "KEY_KEYPOINTS", "- "+strings.Join(article.Summary.Keypoints, "\n- "))
	content = strings.ReplaceAll(content, "KEY_TAGS", "- "+strings.Join(article.Summary.Tags, "\n- "))

	outputPath := filepath.Join(outputFolder, article.Title+".md")

	err := os.WriteFile(outputPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("writing output file: %v", err)
	}

	fmt.Printf("Article created successfully: %s\n", outputPath)
	return nil
}

func isValidWindowsFilename(filename string) bool {
	invalidChars := regexp.MustCompile(`[<>:"/\\|?*\x00-\x1F]`)
	if invalidChars.MatchString(filename) {
		return false
	}

	if len(filename) > 255 || len(filename) == 0 {
		return false
	}

	if strings.HasSuffix(filename, " ") || strings.HasSuffix(filename, ".") {
		return false
	}

	return true
}

func getUserInputtedArticleTitle() string {
	reader := bufio.NewReader(os.Stdin)
	for {
		fmt.Print("Please enter a valid filename: ")
		input, err := reader.ReadString('\n')
		if err != nil {
			fmt.Println("An error occurred while reading input. Please try again", err)
			continue
		}

		// Trim the newline and any spaces
		input = strings.TrimSpace(input)

		if isValidWindowsFilename(input) {
			return input
		} else {
			fmt.Println("The entered filename is still not valid. Please try again.")
		}
	}
}
