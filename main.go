package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

type GitHubFileResponse struct {
	SHA     string `json:"sha"`
	Content string `json:"content"`
}

type UpdateRequest struct {
	Message string `json:"message"`
	Content string `json:"content"`
	SHA     string `json:"sha"`
	Branch  string `json:"branch"`
}

var client = &http.Client{
	Timeout: 15 * time.Second,
}

func main() {
	godotenv.Load()

	token := os.Getenv("GITHUB_TOKEN")
	owner := os.Getenv("OWNER")
	repo := os.Getenv("REPO")
	filePath := os.Getenv("FILE_PATH")
	branch := os.Getenv("BRANCH")

	if token == "" ||
		owner == "" ||
		repo == "" ||
		filePath == "" ||
		branch == "" {
		fmt.Println("Missing .env values")
		return
	}

	fmt.Println("Started commit loop")

	for {
		err := updateFile(token, owner, repo, filePath, branch)

		if err != nil {
			fmt.Println("Error:", err)

			time.Sleep(5 * time.Second)
			continue
		}

		time.Sleep(1500 * time.Millisecond)
	}
}

func updateFile(token, owner, repo, path, branch string) error {
	url := fmt.Sprintf(
		"https://api.github.com/repos/%s/%s/contents/%s",
		owner,
		repo,
		path,
	)

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return err
	}

	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", "github-fast-bot")

	res, err := client.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return err
	}

	if res.StatusCode == 403 {
		fmt.Println("Rate limited, waiting 30 seconds...")
		time.Sleep(30 * time.Second)
		return nil
	}

	if res.StatusCode >= 300 {
		return fmt.Errorf("GET failed: %s", string(body))
	}

	var file GitHubFileResponse

	err = json.Unmarshal(body, &file)
	if err != nil {
		return err
	}

	cleanContent := strings.ReplaceAll(file.Content, "\n", "")
	cleanContent = strings.ReplaceAll(cleanContent, "\r", "")

	decoded, err := base64.StdEncoding.DecodeString(cleanContent)
	if err != nil {
		return err
	}

	content := string(decoded)

	if len(content) <= 1 {
		return fmt.Errorf("file almost empty")
	}

	newContent := content[:len(content)-1]

	encoded := base64.StdEncoding.EncodeToString([]byte(newContent))

	payload := UpdateRequest{
		Message: fmt.Sprintf("auto update %d", time.Now().Unix()),
		Content: encoded,
		SHA:     file.SHA,
		Branch:  branch,
	}

	jsonPayload, err := json.Marshal(payload)
	if err != nil {
		return err
	}

	putReq, err := http.NewRequest(
		"PUT",
		url,
		bytes.NewBuffer(jsonPayload),
	)

	if err != nil {
		return err
	}

	putReq.Header.Set("Authorization", "Bearer "+token)
	putReq.Header.Set("Content-Type", "application/json")
	putReq.Header.Set("User-Agent", "github-fast-bot")

	putRes, err := client.Do(putReq)
	if err != nil {
		return err
	}
	defer putRes.Body.Close()

	responseBody, err := io.ReadAll(putRes.Body)
	if err != nil {
		return err
	}

	if putRes.StatusCode == 403 {
		fmt.Println("Rate limited, waiting 30 seconds...")
		time.Sleep(30 * time.Second)
		return nil
	}

	if putRes.StatusCode >= 300 {
		return fmt.Errorf("PUT failed: %s", string(responseBody))
	}

	fmt.Printf(
		"Committed | removed: %q | remaining: %d chars\n",
		string(content[len(content)-1]),
		len(newContent),
	)

	return nil
}
