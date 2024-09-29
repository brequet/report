# Report CLI Tool

This is a command-line tool written in Go that allows users to scrape an article from a given URL, summarize it using the GROQ API, and export the result to a specified output folder.

## Features

- Scrapes an article from a provided URL.
- Summarizes the article using the GROQ API (`llama-3.1-8b-instant` model).
- Validates and allows renaming of article titles that aren't valid Windows filenames.
- Exports the article and its summary to a specified output folder.

## Requirements

- Go 1.16 or higher.
- Environment variable `GROQ_API_KEY` must be set with a valid API key from GROQ.

## Usage

Run the tool by providing the output folder and article URL as arguments:

```bash
./report <output-folder> <url>
```

Example:

```bash
./report ./articles https://example.com/my-article
```

### Environment Variables

`GROQ_API_KEY`: Your API key for accessing the GROQ API. This should be set in your environment before running the tool.

## How It Works

1. The tool scrapes the article content from the provided URL.
2. It checks if the article title is a valid Windows filename, allowing you to rename it if it's invalid.
3. The article is summarized using the GROQ API by sending a request to:

```bash
https://api.groq.com/openai/v1/chat/completions
```

The `llama-3.1-8b-instant` model is used for generating the summary.

4. Finally, the tool exports the article and its summary to the output folder in the specified format using the template provided in `article-template.md`.
